//go:build !unix

/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"errors"
	"os"
)

// lockFile fails. Two writers on one chain would fork it, and only unix flock
// is implemented, so refuse to start rather than run unlocked.
func lockFile(string) (*os.File, error) {
	return nil, errors.New("receipts: chain locking is only implemented on unix platforms")
}
