/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// tamperServer is a writer that signs with lw, then lets tamper replace the
// receipt it returns. It also serves the real head.
func tamperServer(t *testing.T, lw *LocalWriter, tamper func(n int, rc receiptspec.Receipt) receiptspec.Receipt) *httptest.Server {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/head" {
			seq, h := lw.Head()
			_ = json.NewEncoder(w).Encode(HeadResponse{Seq: seq, EntryHash: hex.EncodeToString(h[:]), SignerKID: lw.KID()})
			return
		}
		var rec DecisionRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		rc, err := lw.Append(r.Context(), rec)
		if err != nil {
			http.Error(w, "append", http.StatusInternalServerError)
			return
		}
		n++
		if tamper != nil {
			rc = tamper(n, rc)
		}
		line, _ := receiptspec.MarshalJSONLReceipt(rc)
		_ = json.NewEncoder(w).Encode(AppendResponse{Receipt: line})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRemoteWriterVerifiesReceipts(t *testing.T) {
	var first receiptspec.Receipt
	tests := []struct {
		name    string
		pin     func(lw *LocalWriter) receiptspec.TrustedKey
		tamper  func(n int, rc receiptspec.Receipt) receiptspec.Receipt
		wantErr []bool
	}{
		{name: "genuine chain", wantErr: []bool{false, false}},
		{name: "forged signature", tamper: func(_ int, rc receiptspec.Receipt) receiptspec.Receipt {
			rc.Signature[0] ^= 0xff
			return rc
		}, wantErr: []bool{true}},
		{name: "forged entry hash", tamper: func(_ int, rc receiptspec.Receipt) receiptspec.Receipt {
			rc.EntryHash[0] ^= 0xff
			return rc
		}, wantErr: []bool{true}},
		{name: "resealed by an attacker key with the pinned kid", tamper: func(_ int, rc receiptspec.Receipt) receiptspec.Receipt {
			forged, _ := receiptspec.Seal(rc, receiptspec.NewEd25519Signer(testKey(9)))
			forged.SignerKID = rc.SignerKID
			return forged
		}, wantErr: []bool{true}},
		{name: "wrong pinned key", pin: func(*LocalWriter) receiptspec.TrustedKey {
			pub := testKey(2).Public().(ed25519.PublicKey)
			return receiptspec.TrustedKey{KID: receiptspec.ComputeKID(pub), PublicKey: pub}
		}, wantErr: []bool{true}},
		{name: "replayed receipt", tamper: func(n int, rc receiptspec.Receipt) receiptspec.Receipt {
			if n == 1 {
				first = rc
				return rc
			}
			return first
		}, wantErr: []bool{false, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lw := openTest(t, t.TempDir(), 1)
			t.Cleanup(func() { _ = lw.Close() })
			srv := tamperServer(t, lw, tc.tamper)
			rw, err := NewRemoteWriter(srv.URL, staticToken("tok"), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			key := lw.TrustedKey()
			if tc.pin != nil {
				key = tc.pin(lw)
			}
			if err := rw.Pin([]receiptspec.TrustedKey{key}); err != nil {
				t.Fatal(err)
			}
			rec := mustRecord(t, testParams())
			for i, want := range tc.wantErr {
				_, err := rw.Append(context.Background(), rec)
				if (err != nil) != want {
					t.Fatalf("append %d: err = %v, wantErr %v", i+1, err, want)
				}
			}
		})
	}
}

func TestRemoteWriterRejectsSeqGap(t *testing.T) {
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	srv := tamperServer(t, lw, nil)
	rw, _ := NewRemoteWriter(srv.URL, staticToken("tok"), time.Second)
	if err := rw.Pin([]receiptspec.TrustedKey{lw.TrustedKey()}); err != nil {
		t.Fatal(err)
	}
	rec := mustRecord(t, testParams())
	if _, err := rw.Append(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	// Another receipt lands on the writer that this client never accepted.
	if _, err := lw.Append(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Append(context.Background(), rec); err == nil {
		t.Fatal("non-continuous seq accepted")
	}
}

func TestRemoteWriterRefusesUnpinned(t *testing.T) {
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	srv := tamperServer(t, lw, nil)
	rw, _ := NewRemoteWriter(srv.URL, staticToken("tok"), time.Second)
	if _, err := rw.Append(context.Background(), mustRecord(t, testParams())); err != ErrUnpinned {
		t.Fatalf("err = %v, want ErrUnpinned", err)
	}
	if err := rw.Pin(nil); err == nil {
		t.Fatal("empty pin accepted")
	}
}

func TestConfigPinsTrustFile(t *testing.T) {
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	trust, err := TrustRootJSON(lw.TrustedKey())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "trust.json")
	if err := os.WriteFile(path, trust, 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"RECEIPTS_ENABLED": "true", "RECEIPTS_WRITER_TRUST_FILE": path, "RECEIPTS_WRITER_URL": "http://w:8080", "RECEIPTS_WRITER_TOKEN_FILE": "/t"}
	c, err := ConfigFromEnv(func(k string) string { return env[k] })
	if err != nil || len(c.TrustedKeys) != 1 {
		t.Fatalf("config = %+v, err %v", c, err)
	}
	if w, err := c.NewWriter(); err != nil || w == nil {
		t.Fatalf("pinned writer not built: %v", err)
	}
	c.TrustedKeys = nil
	if _, err := c.NewWriter(); err == nil {
		t.Fatal("writer built without a pin")
	}
	c.InsecureNoPin = true
	if _, err := c.NewWriter(); err != nil {
		t.Fatalf("insecure demo writer refused: %v", err)
	}
}

func TestTrustRootFromKeyHexMatchesWriter(t *testing.T) {
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	trust, err := TrustRootFromKeyHex(hex.EncodeToString(testKey(1).Seed()))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := receiptspec.LoadTrustedKeys(trust)
	if err != nil || len(keys) != 1 || keys[0].KID != lw.KID() {
		t.Fatalf("keys = %+v, err %v, want kid %s", keys, err, lw.KID())
	}
	if _, err := TrustRootFromKeyHex("zz"); err == nil {
		t.Fatal("bad hex accepted")
	}
}

// A POST can commit on the writer while the client sees an error. The next
// append must resync the head instead of committing another receipt that
// then fails continuity.
func TestRemoteWriterResyncsAfterAmbiguousFailure(t *testing.T) {
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/head" {
			seq, h := lw.Head()
			_ = json.NewEncoder(w).Encode(HeadResponse{Seq: seq, EntryHash: hex.EncodeToString(h[:]), SignerKID: lw.KID()})
			return
		}
		var rec DecisionRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		posts++
		rc, err := lw.Append(r.Context(), rec)
		if err != nil {
			http.Error(w, "append", http.StatusInternalServerError)
			return
		}
		if posts == 2 {
			http.Error(w, "reply lost after commit", http.StatusBadGateway)
			return
		}
		line, _ := receiptspec.MarshalJSONLReceipt(rc)
		_ = json.NewEncoder(w).Encode(AppendResponse{Receipt: line})
	}))
	t.Cleanup(srv.Close)
	rw, _ := NewRemoteWriter(srv.URL, staticToken("tok"), time.Second)
	if err := rw.Pin([]receiptspec.TrustedKey{lw.TrustedKey()}); err != nil {
		t.Fatal(err)
	}
	rec := mustRecord(t, testParams())
	if _, err := rw.Append(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Append(context.Background(), rec); err == nil {
		t.Fatal("lost reply not reported")
	}
	if _, err := rw.Append(context.Background(), rec); err != nil {
		t.Fatalf("append after a lost reply did not resync: %v", err)
	}
	if posts != 3 {
		t.Fatalf("posts = %d, want 3", posts)
	}
}
