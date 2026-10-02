/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// TrustRootFromKeyHex derives the pinned trust file from a hex Ed25519 seed
// (32 bytes) or private key (64 bytes), the format the receipt writer reads.
// Only the public key is written.
func TrustRootFromKeyHex(keyHex string) (json.RawMessage, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, fmt.Errorf("receipts: signing key is not hex")
	}
	priv, err := ParsePrivateKey(raw)
	if err != nil {
		return nil, err
	}
	pub := priv.Public().(ed25519.PublicKey)
	return TrustRootJSON(receiptspec.TrustedKey{KID: receiptspec.ComputeKID(pub), PublicKey: pub})
}
