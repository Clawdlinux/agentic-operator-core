//go:build unix

/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"fmt"
	"os"
	"syscall"
)

// lockFile takes an exclusive, non-blocking flock so only one process writes
// the chain.
func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("receipts: chain already locked by another writer: %w", err)
	}
	return f, nil
}
