/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
)

// ParsePrivateKey builds an Ed25519 private key from a 32-byte seed or a
// 64-byte private key. A 64-byte key is seed plus public half, so the public
// half must match the one derived from the seed. A mismatched key would sign
// receipts that no trust root derived from it can verify.
func ParsePrivateKey(raw []byte) (ed25519.PrivateKey, error) {
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		derived := ed25519.NewKeyFromSeed(raw[:ed25519.SeedSize])
		if !bytes.Equal(derived, raw) {
			return nil, fmt.Errorf("receipts: private key public half does not match its seed")
		}
		return ed25519.PrivateKey(append([]byte(nil), raw...)), nil
	}
	return nil, fmt.Errorf("receipts: signing key must be %d or %d bytes, got %d", ed25519.SeedSize, ed25519.PrivateKeySize, len(raw))
}
