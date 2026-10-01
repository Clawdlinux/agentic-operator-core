/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	cmd.AddCommand(newReceiptsExportCommand(), newReceiptsVerifyCommand(), newReceiptsTrustRootCommand())
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
	var allowPrefix bool
	cmd := &cobra.Command{
		Use:   "verify <export-dir-or-bundle.json>",
		Short: "Verify a receipt export offline",
		Long: "Verifies the receipt chain with the same check agentgate-verify runs, then checks that every " +
			"decision record binds to its receipt. Pass --trust-root with a trust file pinned out of band; " +
			"otherwise the export's own trust.json or embedded keys are used. A pass requires a signed " +
			"export manifest that proves completeness unless --allow-prefix is set.",
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
			return completenessVerdict(stdout, stderr, rep.PrefixOK(), rep.Complete(), allowPrefix)
		},
	}
	cmd.Flags().StringVar(&trustRoot, "trust-root", "", "trust file pinned out of band (receiptspec trust file format)")
	cmd.Flags().BoolVar(&allowPrefix, "allow-prefix", false, allowPrefixHelp)
	return cmd
}

const allowPrefixHelp = "accept an export without a signed manifest; proves only that the receipts present are valid, not that none were removed"

// completenessVerdict turns a verification into an exit status. Without
// allowPrefix a pass needs a signed manifest that proves completeness.
func completenessVerdict(stdout, stderr io.Writer, prefixOK, complete, allowPrefix bool) error {
	if !prefixOK {
		return errVerifyFailed
	}
	if complete {
		fmt.Fprintln(stdout, "completeness: proven against the signed export manifest")
		return nil
	}
	if allowPrefix {
		fmt.Fprintln(stderr, "WARNING: completeness NOT proven. Prefix-only verification (--allow-prefix). Trailing receipts or records may have been removed.")
		return nil
	}
	fmt.Fprintln(stderr, "FAIL: completeness not proven: no signed export manifest matched the chain head. Use --allow-prefix to accept a prefix.")
	return errVerifyFailed
}

func newReceiptsTrustRootCommand() *cobra.Command {
	var keyFile string
	cmd := &cobra.Command{
		Use:   "trust-root",
		Short: "Print the pinned trust file for a receipt-writer signing key",
		Long: "Reads the hex Ed25519 signing key the receipt-writer uses and prints the trust file " +
			"(public key only) for RECEIPTS_WRITER_TRUST_FILE and --trust-root. Run it where the key already lives.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := os.ReadFile(keyFile)
			if err != nil {
				return err
			}
			trust, err := receipts.TrustRootFromKeyHex(string(data))
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(trust))
			return err
		},
	}
	cmd.Flags().StringVar(&keyFile, "signing-key-file", "", "file holding the hex Ed25519 signing key")
	_ = cmd.MarkFlagRequired("signing-key-file")
	return cmd
}
