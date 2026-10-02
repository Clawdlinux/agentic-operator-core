/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

func TestParsePrivateKey(t *testing.T) {
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	full := ed25519.NewKeyFromSeed(seed)

	fromSeed, err := ParsePrivateKey(seed)
	if err != nil || !bytes.Equal(fromSeed, full) {
		t.Fatalf("seed: key %x err %v", fromSeed, err)
	}
	fromFull, err := ParsePrivateKey(full)
	if err != nil || !bytes.Equal(fromFull, full) {
		t.Fatalf("full key: %v", err)
	}

	bad := append([]byte(nil), full...)
	bad[len(bad)-1] ^= 0xff
	if _, err := ParsePrivateKey(bad); err == nil {
		t.Fatal("accepted a key whose public half does not match the seed")
	}
	if _, err := ParsePrivateKey(make([]byte, 10)); err == nil {
		t.Fatal("accepted a wrong-length key")
	}
}
