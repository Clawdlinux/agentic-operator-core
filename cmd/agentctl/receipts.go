/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// errVerifyFailed makes agentctl exit nonzero after FAIL lines are printed.
var errVerifyFailed = errors.New("receipt verification failed")

func newReceiptsCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receipts",
		Short: "Export and verify signed decision receipts",
		// Receipts work offline. Do not load a kubeconfig.
		PersistentPreRunE: func(*cobra.Command, []string) error { return opts.validateOutput() },
	}
	cmd.AddCommand(newReceiptsExportCommand(), newReceiptsVerifyCommand())
	return cmd
}

func newReceiptsExportCommand() *cobra.Command {
	var writer, tokenFile, out string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Download the receipt chain, trust root, and decision records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rw, err := receipts.NewRemoteWriter(writer, receipts.TokenFile(tokenFile), timeout)
			if err != nil {
				return err
			}
			e, err := rw.Export(cmd.Context())
			if err != nil {
				return err
			}
			if err := e.WriteDir(out); err != nil {
				return err
			}
			b, err := receiptspec.ReadJSONL(bytes.NewReader([]byte(e.ReceiptsJSONL)))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported %d receipts to %s (%s, %s, %s)\n",
				len(b.Receipts), out, receipts.ReceiptsFile, receipts.TrustFile, receipts.RecordsFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&writer, "writer", "", "receipt-writer base URL, for example http://localhost:8080")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "file holding the receipt-writer bearer token")
	cmd.Flags().StringVar(&out, "out", "", "output directory")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "HTTP timeout")
	for _, f := range []string{"writer", "token-file", "out"} {
		_ = cmd.MarkFlagRequired(f)
	}
	return cmd
}

func newReceiptsVerifyCommand() *cobra.Command {
	var trustRoot string
	cmd := &cobra.Command{
		Use:   "verify <export-dir-or-bundle.json>",
		Short: "Verify a receipt export offline",
		Long: "Verifies the receipt chain with the same check agentgate-verify runs, then checks that every " +
			"decision record binds to its receipt. Pass --trust-root with a trust file pinned out of band; " +
			"otherwise the export's own trust.json or embedded keys are used.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var trusted []receiptspec.TrustedKey
			if trustRoot != "" {
				data, err := os.ReadFile(trustRoot)
				if err != nil {
					return err
				}
				if trusted, err = receiptspec.LoadTrustedKeys(data); err != nil {
					return err
				}
			}
			e, err := receipts.ReadExport(args[0])
			if err != nil {
				return err
			}
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
			rep, err := receipts.VerifyExport(e, trusted)
			if receipts.IsManifestError(err) {
				fmt.Fprintf(stderr, "FAIL: manifest: %v\n", err)
				return errVerifyFailed
			}
			if err != nil {
				return err
			}
			c := rep.Chain
			if c.OK {
				fmt.Fprintf(stdout, "PASS: %d receipts verified, head seq=%d hash=%x\n", c.VerifiedCount, c.HeadSeq, c.HeadEntryHash[:8])
				if c.Complete {
					fmt.Fprintln(stdout, "completeness: proven against the signed export manifest")
				} else {
					fmt.Fprintln(stdout, "completeness: not claimed")
				}
			} else {
				fmt.Fprintf(stderr, "FAIL: seq=%d reason=%s (%d of %d receipts verified before failure)\n",
					c.FailedAtSeq, c.Reason, c.VerifiedCount, c.TotalReceipts)
			}
			for _, f := range rep.RecordFailures {
				fmt.Fprintf(stderr, "FAIL: record seq=%d reason=%s\n", f.Seq, f.Reason)
			}
			if len(rep.RecordFailures) == 0 {
				fmt.Fprintf(stdout, "PASS: %d decision records bound to their receipts\n", rep.RecordsBound)
			}
			if !rep.OK() {
				return errVerifyFailed
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&trustRoot, "trust-root", "", "trust file pinned out of band (receiptspec trust file format)")
	return cmd
}
