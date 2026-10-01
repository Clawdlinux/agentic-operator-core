/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// LookupFunc returns the receipt and canonical record stored at seq.
type LookupFunc func(seq uint64) (receiptspec.Receipt, json.RawMessage, error)

// Store appends examples to approvals.jsonl. It lives in the receipt-writer
// process, which already holds the data dir lock, so one Store per dir.
type Store struct {
	mu     sync.Mutex
	path   string
	f      *os.File
	lookup LookupFunc
	seqs   map[uint64]bool
	ids    map[string]bool
	broken bool
}

// Open opens or creates approvals.jsonl in dir. A torn last line is cut.
func Open(dir string, lookup LookupFunc) (*Store, error) {
	if lookup == nil {
		return nil, errors.New("dataset: lookup is required")
	}
	path := filepath.Join(dir, ApprovalsFile)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	keep := bytes.LastIndexByte(data, '\n') + 1
	s := &Store{path: path, lookup: lookup, seqs: map[uint64]bool{}, ids: map[string]bool{}}
	for i, line := range bytes.Split(data[:keep], []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var ex Example
		if err := json.Unmarshal(line, &ex); err != nil {
			return nil, fmt.Errorf("dataset: %s line %d: %w", ApprovalsFile, i+1, err)
		}
		s.seqs[ex.ReceiptSeq], s.ids[ex.ExampleID] = true, true
	}
	if keep < len(data) {
		if err := os.Truncate(path, int64(keep)); err != nil {
			return nil, err
		}
	}
	if s.f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err != nil {
		return nil, err
	}
	return s, nil
}

// Append validates ex against its receipt and stores it, fsynced. A second
// example for the same receipt or pending id returns ErrDuplicate.
func (s *Store) Append(ex Example) error {
	r, rec, err := s.lookup(ex.ReceiptSeq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnbound, err)
	}
	if err := Validate(ex, r, rec); err != nil {
		return err
	}
	line, err := json.Marshal(ex)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return errors.New("dataset: store broken by an earlier write failure, restart to repair")
	}
	if s.seqs[ex.ReceiptSeq] || s.ids[ex.ExampleID] {
		return ErrDuplicate
	}
	if _, err = s.f.Write(append(line, '\n')); err == nil {
		err = s.f.Sync()
	}
	if err != nil {
		s.broken = true
		return fmt.Errorf("dataset: store example: %w", err)
	}
	s.seqs[ex.ReceiptSeq], s.ids[ex.ExampleID] = true, true
	return nil
}

// Export returns the whole approvals.jsonl.
func (s *Store) Export() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// Close closes the file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}
