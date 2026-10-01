package dataclass

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// verhoeffAppend returns base plus its Verhoeff check digit.
func verhoeffAppend(base string) string {
	for d := byte('0'); d <= '9'; d++ {
		if verhoeffValid(base + string(d)) {
			return base + string(d)
		}
	}
	panic("no check digit")
}

func TestDetect(t *testing.T) {
	aadhaar := verhoeffAppend("23412341234")
	// Flip the check digit to break Verhoeff.
	badAadhaar := aadhaar[:11] + string('0'+(aadhaar[11]-'0'+1)%10)

	tests := []struct {
		name string
		in   string
		want []Class
	}{
		{"empty", "", nil},
		{"plain text", "restart the deployment in namespace prod", nil},
		{"email", "contact ops.team+alerts@example.co.uk now", []Class{Email}},
		{"not email", "user at example dot com", nil},
		{"phone e164", "call +91 98765 43210", []Class{Phone}},
		{"phone india bare", "mobile 9876543210 today", []Class{Phone}},
		{"phone us", "(415) 555-0100", []Class{Phone}},
		{"short number not phone", "replicas 12345", nil},
		{"aadhaar plain", "id " + aadhaar, []Class{Aadhaar}},
		{"aadhaar spaced", "id " + aadhaar[:4] + " " + aadhaar[4:8] + " " + aadhaar[8:], []Class{Aadhaar}},
		{"aadhaar hyphen", aadhaar[:4] + "-" + aadhaar[4:8] + "-" + aadhaar[8:], []Class{Aadhaar}},
		{"aadhaar bad checksum", "id " + badAadhaar, nil},
		{"aadhaar first digit 1", verhoeffAppend("13412341234"), nil},
		{"pan", "PAN ABCDE1234F on file", []Class{PAN}},
		{"pan lowercase no match", "abcde1234f", nil},
		{"iban compact", "send to GB82WEST12345698765432", []Class{IBAN}},
		{"iban spaced", "GB82 WEST 1234 5698 7654 32 THANKS", []Class{IBAN}},
		{"iban de", "DE89370400440532013000", []Class{IBAN}},
		{"iban bad checksum", "GB83WEST12345698765432", nil},
		{"card", "4111111111111111", []Class{Card}},
		{"card spaced", "card 4111 1111 1111 1111 exp", []Class{Card}},
		{"card hyphen", "5500-0000-0000-0004", []Class{Card}},
		{"card bad luhn", "4111111111111112", nil},
		{"multi", "a@b.io and 4111111111111111", []Class{Card, Email}},
		{"dedupe", "a@b.io c@d.io", []Class{Email}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Detect(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Detect = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectRandomTwelveDigitsFailingVerhoeff(t *testing.T) {
	// Every check digit except the valid one must not match aadhaar.
	base := "98765432109"
	valid := verhoeffAppend(base)
	for d := byte('0'); d <= '9'; d++ {
		s := base + string(d)
		has := false
		for _, c := range Detect(s) {
			if c == Aadhaar {
				has = true
			}
		}
		if has != (s == valid) {
			t.Fatalf("%s: aadhaar match = %v, want %v", s[:4]+"...", has, s == valid)
		}
	}
}

func TestDetectValue(t *testing.T) {
	var decoded any
	if err := json.Unmarshal([]byte(`{"to":"x@y.com","items":[{"pan":"ABCDE1234F"}],"n":4111111111111111}`), &decoded); err != nil {
		t.Fatal(err)
	}

	deep := any("a@b.io")
	for i := 0; i < MaxDepth+2; i++ {
		deep = []any{deep}
	}

	tests := []struct {
		name string
		in   any
		want []Class
	}{
		{"nil", nil, nil},
		{"string", "a@b.io", []Class{Email}},
		{"decoded json", decoded, []Class{Card, Email, PAN}},
		{"json number", json.Number("4111111111111111"), []Class{Card}},
		{"keys not scanned", map[string]any{"a@b.io": "ok"}, nil},
		{"string map", map[string]string{"k": "ABCDE1234F"}, []Class{PAN}},
		{"string slice", []string{"x", "GB82WEST12345698765432"}, []Class{IBAN}},
		{"too deep", deep, nil},
		{"unsupported type", struct{ S string }{"a@b.io"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectValue(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DetectValue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectValueSizeLimited(t *testing.T) {
	big := strings.Repeat("x", MaxTotalBytes)
	if got := DetectValue([]any{big, "a@b.io"}); len(got) != 0 {
		t.Fatalf("budget not enforced: %v", got)
	}
	long := strings.Repeat("y", MaxStringBytes) + " a@b.io"
	if got := Detect(long); len(got) != 0 {
		t.Fatalf("string cap not enforced: %v", got)
	}
}

func TestScanReportsIncomplete(t *testing.T) {
	deep := any("AKIAABCDEFGHIJKLMNOP")
	for i := 0; i <= MaxDepth+1; i++ {
		deep = []any{deep}
	}
	manyNodes := make([]any, MaxNodes+1)
	for i := range manyNodes {
		manyNodes[i] = "x"
	}
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"small", map[string]any{"a": "b", "n": 1.0, "t": true, "z": nil}, true},
		{"string past per-string cap", strings.Repeat("y", MaxStringBytes) + " AKIAABCDEFGHIJKLMNOP", false},
		{"total budget exhausted", []any{strings.Repeat("x", MaxTotalBytes), "a@b.io"}, false},
		{"too deep", deep, false},
		{"too many nodes", manyNodes, false},
		{"unknown leaf type", []any{struct{ S string }{"a@b.io"}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := Scan(tc.v); got != tc.want {
				t.Fatalf("complete = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectCredential(t *testing.T) {
	// Test fixtures only. None of these are live secrets.
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"aws key id", "key AKIAIOSFODNN7EXAMPLE here", true},
		{"aws too short", "AKIAIOSFODNN7EXAMP", false},
		{"aws lowercase", "akiaiosfodnn7example", false},
		{"rsa private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIE", true},
		{"generic private key", "-----BEGIN PRIVATE KEY-----", true},
		{"public key", "-----BEGIN PUBLIC KEY-----", false},
		{"jwt", "token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig", true},
		{"jwt one part", "eyJhbGciOiJIUzI1NiJ9 only", false},
		{"bearer", "Authorization: Bearer abcdefghijklmnopqrstuvwxyz012345", true},
		{"bearer short", "bearer abc", false},
		{"github classic", "ghp_" + strings.Repeat("a1", 18), true},
		{"github fine grained", "github_pat_" + strings.Repeat("A", 30), true},
		{"github prefix only", "ghp_short", false},
		{"plain", "scale deployment web to 3 replicas", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := false
			for _, c := range Detect(tc.in) {
				if c == Credential {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("credential = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsPersonal(t *testing.T) {
	for _, c := range []string{"email", "PHONE", "aadhaar", "pan", "iban", "card"} {
		if !IsPersonal(c) {
			t.Fatalf("%s should be personal", c)
		}
	}
	for _, c := range []string{"credential", "", "other"} {
		if IsPersonal(c) {
			t.Fatalf("%s should not be personal", c)
		}
	}
}

func TestStrings(t *testing.T) {
	if got := Strings([]Class{Card, Email}); !reflect.DeepEqual(got, []string{"card", "email"}) {
		t.Fatalf("Strings = %v", got)
	}
}
