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

// Package packs ships versioned Rego policy packs. Packs are engineering
// controls. They are not legal advice and not a certification. This package
// has no OPA dependency so the API webhook can validate pack names cheaply.
// See docs/policy-packs.md.
package packs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed dpdp-in@v0.1.0 gdpr-eu@v0.1.0
var files embed.FS

// NamePattern is the CRD pattern for one spec.policyPacks entry.
const NamePattern = `^(dpdp-in|gdpr-eu)@v[0-9]+\.[0-9]+\.[0-9]+$`

// Known returns every shipped pack, sorted.
func Known() []string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Valid reports whether name is a shipped pack at an exact version.
func Valid(name string) bool {
	for _, k := range Known() {
		if k == name {
			return true
		}
	}
	return false
}

// FS returns the files of one pack. Unknown names return an error.
func FS(name string) (fs.FS, error) {
	if !Valid(name) {
		return nil, fmt.Errorf("packs: unknown policy pack %q, known: %v", name, Known())
	}
	return fs.Sub(files, name)
}
