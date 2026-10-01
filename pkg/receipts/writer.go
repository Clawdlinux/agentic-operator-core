/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"context"
	"errors"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// Writer appends decision receipts. Append must store the receipt durably
// before it returns, because the caller acts on it.
type Writer interface {
	Append(ctx context.Context, rec DecisionRecord) (receiptspec.Receipt, error)
	Ready(ctx context.Context) bool
}

// ErrWriterBroken means a previous write failed part way. The writer refuses
// further appends until it is reopened, which repairs the files.
var ErrWriterBroken = errors.New("receipts: writer broken by an earlier write failure, reopen to repair")
