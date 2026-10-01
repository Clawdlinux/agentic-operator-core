/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package dataclass runs deterministic detectors for personal, financial, and
// credential data classes. It returns class names only. It never returns or
// logs the matched content.
package dataclass

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Class is a detected data class name.
type Class string

// Supported data classes.
const (
	Email   Class = "email"
	Phone   Class = "phone"
	Aadhaar Class = "aadhaar"
	PAN     Class = "pan"
	IBAN    Class = "iban"
	Card    Class = "card"
	// Credential marks a secret-shaped string. Invariant INV-01 always denies it.
	Credential Class = "credential"
)

// Limits keep scanning bounded on hostile input.
const (
	// MaxDepth is the deepest nesting DetectValue descends into.
	MaxDepth = 8
	// MaxStringBytes is the prefix of each string that is scanned.
	MaxStringBytes = 64 << 10
	// MaxTotalBytes is the total string budget for one DetectValue call.
	MaxTotalBytes = 1 << 20
	// MaxNodes is the node budget for one DetectValue call.
	MaxNodes = 10000
)

var (
	emailRe     = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
	phoneIntlRe = regexp.MustCompile(`\+[0-9](?:[ -]?[0-9]){7,14}`)
	phoneINRe   = regexp.MustCompile(`\b[6-9][0-9]{9}\b`)
	phoneUSRe   = regexp.MustCompile(`\([0-9]{3}\) ?[0-9]{3}-[0-9]{4}`)
	aadhaarRe   = regexp.MustCompile(`\b[2-9][0-9]{3}[ -]?[0-9]{4}[ -]?[0-9]{4}\b`)
	panRe       = regexp.MustCompile(`\b[A-Z]{5}[0-9]{4}[A-Z]\b`)
	ibanRe      = regexp.MustCompile(`\b[A-Z]{2}[0-9]{2}(?: ?[A-Z0-9]){11,30}\b`)
	cardRe      = regexp.MustCompile(`\b[0-9](?:[ -]?[0-9]){12,18}\b`)

	credentialRes = []*regexp.Regexp{
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`),
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{4,}\.eyJ[A-Za-z0-9_-]{4,}`),
		regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]{20,}`),
		regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,}`),
	}
)

// Detect returns the sorted, deduplicated classes found in s.
func Detect(s string) []Class {
	set := map[Class]struct{}{}
	detectInto(s, set)
	return sorted(set)
}

// DetectValue walks JSON-like values (map[string]any, []any, strings, numbers)
// and returns the sorted, deduplicated classes found in string and numeric
// leaves. Map keys are not scanned.
func DetectValue(v any) []Class {
	set := map[Class]struct{}{}
	b := &budget{bytes: MaxTotalBytes, nodes: MaxNodes}
	walk(v, 0, b, set)
	return sorted(set)
}

type budget struct {
	bytes int
	nodes int
}

func walk(v any, depth int, b *budget, set map[Class]struct{}) {
	if depth > MaxDepth || b.nodes <= 0 || b.bytes <= 0 {
		return
	}
	b.nodes--
	switch t := v.(type) {
	case string:
		if len(t) > b.bytes {
			t = t[:b.bytes]
		}
		b.bytes -= len(t)
		detectInto(t, set)
	case json.Number:
		walk(t.String(), depth, b, set)
	case int, int32, int64, uint, uint32, uint64:
		walk(fmt.Sprint(t), depth, b, set)
	case float64:
		// Large integers survive as float64 from encoding/json without UseNumber.
		walk(fmt.Sprintf("%.0f", t), depth, b, set)
	case []any:
		for _, e := range t {
			walk(e, depth+1, b, set)
		}
	case []string:
		for _, e := range t {
			walk(e, depth+1, b, set)
		}
	case map[string]any:
		for _, k := range sortedKeys(t) {
			walk(t[k], depth+1, b, set)
		}
	case map[string]string:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(t[k], depth+1, b, set)
		}
	}
}

func detectInto(s string, set map[Class]struct{}) {
	if len(s) > MaxStringBytes {
		s = s[:MaxStringBytes]
	}
	if emailRe.MatchString(s) {
		set[Email] = struct{}{}
	}
	if phoneIntlRe.MatchString(s) || phoneUSRe.MatchString(s) || anyIsolated(s, phoneINRe, nil) {
		set[Phone] = struct{}{}
	}
	if anyIsolated(s, aadhaarRe, func(d string) bool { return verhoeffValid(d) }) {
		set[Aadhaar] = struct{}{}
	}
	if panRe.MatchString(s) {
		set[PAN] = struct{}{}
	}
	if anyIBAN(s) {
		set[IBAN] = struct{}{}
	}
	if anyIsolated(s, cardRe, luhnValid) {
		set[Card] = struct{}{}
	}
	for _, re := range credentialRes {
		if re.MatchString(s) {
			set[Credential] = struct{}{}
			break
		}
	}
}

// anyIsolated reports whether some match of re stands alone (no adjacent digit
// group) and, after removing separators, passes check (nil means accept).
func anyIsolated(s string, re *regexp.Regexp, check func(string) bool) bool {
	for _, loc := range re.FindAllStringIndex(s, -1) {
		if !isolated(s, loc[0], loc[1]) {
			continue
		}
		if check == nil || check(digitsOnly(s[loc[0]:loc[1]])) {
			return true
		}
	}
	return false
}

func isolated(s string, start, end int) bool {
	if start > 0 {
		p := s[start-1]
		if isDigit(p) {
			return false
		}
		if (p == ' ' || p == '-') && start > 1 && isDigit(s[start-2]) {
			return false
		}
	}
	if end < len(s) {
		n := s[end]
		if isDigit(n) {
			return false
		}
		if (n == ' ' || n == '-') && end+1 < len(s) && isDigit(s[end+1]) {
			return false
		}
	}
	return true
}

func anyIBAN(s string) bool {
	for _, m := range ibanRe.FindAllString(s, -1) {
		// Greedy matching can swallow a following uppercase word, so try each
		// group boundary from longest to shortest.
		cut := len(m)
		for cut > 0 {
			if ibanValid(strings.ReplaceAll(m[:cut], " ", "")) {
				return true
			}
			i := strings.LastIndexByte(m[:cut], ' ')
			if i < 0 {
				break
			}
			cut = i
		}
	}
	return false
}

func ibanValid(s string) bool {
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	r := s[4:] + s[:4]
	rem := 0
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case isDigit(c):
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			v := int(c-'A') + 10
			rem = (rem*100 + v) % 97
		default:
			return false
		}
	}
	return rem == 1
}

func luhnValid(d string) bool {
	if len(d) < 13 || len(d) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}

var verhoeffD = [10][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 2, 3, 4, 0, 6, 7, 8, 9, 5},
	{2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
	{3, 4, 0, 1, 2, 8, 9, 5, 6, 7},
	{4, 0, 1, 2, 3, 9, 5, 6, 7, 8},
	{5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
	{6, 5, 9, 8, 7, 1, 0, 4, 3, 2},
	{7, 6, 5, 9, 8, 2, 1, 0, 4, 3},
	{8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}

var verhoeffP = [8][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 5, 7, 6, 2, 8, 3, 0, 9, 4},
	{5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
	{8, 9, 1, 6, 0, 4, 3, 5, 2, 7},
	{9, 4, 5, 3, 1, 2, 6, 8, 7, 0},
	{4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
	{2, 7, 9, 3, 8, 0, 6, 4, 1, 5},
	{7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
}

func verhoeffValid(d string) bool {
	if len(d) != 12 {
		return false
	}
	c := 0
	for i := 0; i < len(d); i++ {
		c = verhoeffD[c][verhoeffP[i%8][int(d[len(d)-1-i]-'0')]]
	}
	return c == 0
}

func digitsOnly(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if isDigit(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sorted(set map[Class]struct{}) []Class {
	out := make([]Class, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsPersonal reports whether name is a personal data class. Credential is not
// personal data; INV-01 handles it.
func IsPersonal(name string) bool {
	switch Class(strings.ToLower(name)) {
	case Email, Phone, Aadhaar, PAN, IBAN, Card:
		return true
	}
	return false
}

// Strings converts classes to plain strings.
func Strings(cs []Class) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = string(c)
	}
	return out
}
