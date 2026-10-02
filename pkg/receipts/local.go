/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// File names inside a LocalWriter data dir and an export dir.
const (
	ReceiptsFile = "receipts.jsonl"
	RecordsFile  = "records.jsonl"
	TrustFile    = "trust.json"
	lockName     = "writer.lock"
)

// ErrClosed means the writer was closed.
var ErrClosed = errors.New("receipts: writer closed")

// RecordLine is one line of records.jsonl: the decision record bound to the
// receipt with the same seq.
type RecordLine struct {
	Seq    uint64          `json:"seq"`
	Record json.RawMessage `json:"record"`
}

// LocalWriter appends signed receipts to receipts.jsonl and decision records
// to records.jsonl in one directory. It is the only writer of that directory:
// a flock guards it across processes and a mutex inside the process.
//
// Each Append writes and fsyncs the record first, then the receipt. On open,
// a torn last line is cut, and records without a receipt are dropped, so a
// crash never leaves a receipt without its record.
type LocalWriter struct {
	mu       sync.Mutex
	priv     ed25519.PrivateKey
	kid      string
	chain    *receiptspec.Chain
	dir      string
	receipts *os.File
	records  *os.File
	lock     *os.File
	broken   bool
	closed   bool
}

// OpenLocal opens or creates the chain in dir and resumes from its last
// receipt. It refuses to open a chain that is internally inconsistent or was
// signed by a different key. Key rotation is not supported yet.
func OpenLocal(dir string, priv ed25519.PrivateKey) (*LocalWriter, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("receipts: invalid ed25519 private key")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := lockFile(filepath.Join(dir, lockName))
	if err != nil {
		return nil, err
	}
	w := &LocalWriter{priv: priv, dir: dir, lock: lock}
	w.kid = receiptspec.ComputeKID(priv.Public().(ed25519.PublicKey))
	if err := w.open(); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return w, nil
}

func (w *LocalWriter) open() error {
	rPath := filepath.Join(w.dir, ReceiptsFile)
	data, err := readOptional(rPath)
	if err != nil {
		return err
	}
	lines, keep := completeLines(data)
	var (
		seq     uint64
		head    [32]byte
		lastKID string
	)
	for i, line := range lines {
		r, err := receiptspec.ParseJSONLReceipt(line)
		if err != nil {
			return fmt.Errorf("receipts: %s line %d: %w", ReceiptsFile, i+1, err)
		}
		hash, err := receiptspec.ComputeEntryHash(r)
		if err != nil || hash != r.EntryHash || r.Seq != seq+1 || r.PrevHash != head {
			return fmt.Errorf("receipts: %s line %d: chain inconsistent, refusing to resume", ReceiptsFile, i+1)
		}
		seq, head, lastKID = r.Seq, r.EntryHash, r.SignerKID
	}
	if seq > 0 && lastKID != w.kid {
		return fmt.Errorf("receipts: chain head signed by %s, key is %s: key rotation is not supported", lastKID, w.kid)
	}
	if err := truncateTo(rPath, keep, len(data)); err != nil {
		return err
	}

	recPath := filepath.Join(w.dir, RecordsFile)
	recData, err := readOptional(recPath)
	if err != nil {
		return err
	}
	recLines, recKeep := completeLines(recData)
	offset := 0
	for i, line := range recLines {
		var rl RecordLine
		if err := json.Unmarshal(line, &rl); err != nil {
			return fmt.Errorf("receipts: %s line %d: %w", RecordsFile, i+1, err)
		}
		if rl.Seq > seq {
			recKeep = offset
			break
		}
		offset += len(line) + 1
	}
	if err := truncateTo(recPath, recKeep, len(recData)); err != nil {
		return err
	}

	if w.receipts, err = os.OpenFile(rPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err != nil {
		return err
	}
	if w.records, err = os.OpenFile(recPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err != nil {
		_ = w.receipts.Close()
		return err
	}
	syncDir(w.dir)
	w.chain = receiptspec.NewChain(receiptspec.NewEd25519Signer(w.priv), seq, head)
	return nil
}

// Append signs and stores one receipt and its record, fsynced, before
// returning. Any write failure marks the writer broken.
func (w *LocalWriter) Append(ctx context.Context, rec DecisionRecord) (receiptspec.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return receiptspec.Receipt{}, err
	}
	canonical, err := receiptspec.CanonicalRecord(rec)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	fields, err := Fields(rec)
	if err != nil {
		return receiptspec.Receipt{}, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return receiptspec.Receipt{}, ErrClosed
	}
	if w.broken {
		return receiptspec.Receipt{}, ErrWriterBroken
	}
	r, err := w.chain.Next(fields)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	recLine, err := json.Marshal(RecordLine{Seq: r.Seq, Record: canonical})
	if err == nil {
		err = appendSync(w.records, recLine)
	}
	if err == nil {
		var line []byte
		if line, err = receiptspec.MarshalJSONLReceipt(r); err == nil {
			err = appendSync(w.receipts, line)
		}
	}
	if err != nil {
		w.broken = true
		return receiptspec.Receipt{}, fmt.Errorf("receipts: store receipt seq %d: %w", r.Seq, err)
	}
	return r, nil
}

// Ready reports whether Append can be called.
func (w *LocalWriter) Ready(context.Context) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.closed && !w.broken
}

// Head returns the last stored (seq, entry_hash).
func (w *LocalWriter) Head() (uint64, [32]byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.chain.Head()
}

// ErrNoSuchReceipt means Lookup found no receipt with that seq.
var ErrNoSuchReceipt = errors.New("receipts: no receipt with that seq")

// Lookup returns the stored receipt and canonical record for seq. It scans the
// files, so it costs O(chain length).
func (w *LocalWriter) Lookup(seq uint64) (receiptspec.Receipt, json.RawMessage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return receiptspec.Receipt{}, nil, ErrClosed
	}
	if head, _ := w.chain.Head(); seq == 0 || seq > head {
		return receiptspec.Receipt{}, nil, ErrNoSuchReceipt
	}
	data, err := readOptional(filepath.Join(w.dir, ReceiptsFile))
	if err != nil {
		return receiptspec.Receipt{}, nil, err
	}
	lines, _ := completeLines(data)
	if int(seq) > len(lines) {
		return receiptspec.Receipt{}, nil, ErrNoSuchReceipt
	}
	r, err := receiptspec.ParseJSONLReceipt(lines[seq-1])
	if err != nil || r.Seq != seq {
		return receiptspec.Receipt{}, nil, fmt.Errorf("receipts: receipt seq %d unreadable", seq)
	}
	recData, err := readOptional(filepath.Join(w.dir, RecordsFile))
	if err != nil {
		return receiptspec.Receipt{}, nil, err
	}
	recLines, _ := completeLines(recData)
	for _, line := range recLines {
		var rl RecordLine
		if json.Unmarshal(line, &rl) == nil && rl.Seq == seq {
			return r, rl.Record, nil
		}
	}
	return receiptspec.Receipt{}, nil, fmt.Errorf("receipts: record for seq %d missing", seq)
}

// KID returns the signer key ID.
func (w *LocalWriter) KID() string { return w.kid }

// TrustedKey returns the public key entry for verifiers.
func (w *LocalWriter) TrustedKey() receiptspec.TrustedKey {
	return receiptspec.TrustedKey{KID: w.kid, PublicKey: w.priv.Public().(ed25519.PublicKey)}
}

// Close releases the files and the lock.
func (w *LocalWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return errors.Join(w.receipts.Close(), w.records.Close(), w.lock.Close())
}

// Export builds a full, signed export of the chain plus its records.
func (w *LocalWriter) Export() (Export, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return Export{}, ErrClosed
	}
	data, err := readOptional(filepath.Join(w.dir, ReceiptsFile))
	if err != nil {
		return Export{}, err
	}
	b, err := receiptspec.ReadJSONL(bytes.NewReader(data))
	if err != nil {
		return Export{}, err
	}
	records, err := readOptional(filepath.Join(w.dir, RecordsFile))
	if err != nil {
		return Export{}, err
	}
	key := receiptspec.TrustedKey{KID: w.kid, PublicKey: w.priv.Public().(ed25519.PublicKey)}
	headSeq, headHash := w.chain.Head()
	m := receiptspec.ExportManifest{
		FormatVersion: receiptspec.ExportFormatVersion,
		RequestedFrom: 1,
		ResolvedTo:    headSeq,
		Count:         len(b.Receipts),
		HeadSeq:       headSeq,
		HeadHash:      headHash,
		KeysetDigest:  receiptspec.ComputeKeysetDigest([]receiptspec.TrustedKey{key}),
	}
	if n := len(b.Receipts); n > 0 {
		m.FirstEntryHash = b.Receipts[0].EntryHash
		m.LastEntryHash = b.Receipts[n-1].EntryHash
	}
	m = receiptspec.SignManifest(m, w.kid, w.priv)

	var buf bytes.Buffer
	for _, line := range []func() ([]byte, error){
		func() ([]byte, error) { return receiptspec.MarshalManifestLine(m) },
		func() ([]byte, error) { return receiptspec.MarshalKeyLine(key) },
	} {
		l, err := line()
		if err != nil {
			return Export{}, err
		}
		buf.Write(append(l, '\n'))
	}
	if err := receiptspec.WriteJSONL(&buf, b.Receipts); err != nil {
		return Export{}, err
	}
	trust, err := TrustRootJSON(key)
	if err != nil {
		return Export{}, err
	}
	return Export{
		Format:        ExportFormat,
		ReceiptsJSONL: buf.String(),
		TrustRoot:     trust,
		RecordsJSONL:  string(records),
	}, nil
}

// TrustRootJSON renders keys in the trust file shape receiptspec.LoadTrustedKeys reads.
func TrustRootJSON(keys ...receiptspec.TrustedKey) (json.RawMessage, error) {
	type k struct {
		KID           string  `json:"kid"`
		PublicKeyHex  string  `json:"public_key_hex"`
		ValidFromSeq  uint64  `json:"valid_from_seq"`
		ValidUntilSeq *uint64 `json:"valid_until_seq"`
	}
	out := struct {
		Keys []k `json:"keys"`
	}{}
	for _, key := range keys {
		out.Keys = append(out.Keys, k{KID: key.KID, PublicKeyHex: hex.EncodeToString(key.PublicKey), ValidFromSeq: key.ValidFromSeq, ValidUntilSeq: key.ValidUntilSeq})
	}
	return json.Marshal(out)
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// completeLines returns every newline-terminated line, without the newline,
// and the byte length they cover. A trailing partial line is not included.
func completeLines(data []byte) ([][]byte, int) {
	end := bytes.LastIndexByte(data, '\n') + 1
	var lines [][]byte
	for _, l := range bytes.SplitAfter(data[:end], []byte{'\n'}) {
		if len(l) > 0 {
			lines = append(lines, l[:len(l)-1])
		}
	}
	return lines, end
}

func truncateTo(path string, keep, size int) error {
	if keep >= size {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := f.Truncate(int64(keep)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func appendSync(f *os.File, line []byte) error {
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}
