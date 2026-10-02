/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

func newDatasetCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Export and verify the signed approval dataset",
		// The dataset works offline. Do not load a kubeconfig.
		PersistentPreRunE: func(*cobra.Command, []string) error { return opts.validateOutput() },
	}
	cmd.AddCommand(newDatasetExportCommand(), newDatasetVerifyCommand())
	return cmd
}

func newDatasetExportCommand() *cobra.Command {
	var writer, tokenFile, out string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Download approvals.jsonl with the receipt chain it binds to",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rw, err := receipts.NewRemoteWriter(writer, receipts.TokenFile(tokenFile), timeout)
			if err != nil {
				return err
			}
			ds, err := dataset.NewRemote(writer, receipts.TokenFile(tokenFile), timeout)
			if err != nil {
				return err
			}
			// Approvals first: every example then has its receipt in the later chain export.
			approvals, err := ds.Export(cmd.Context())
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
			if err := os.WriteFile(filepath.Join(out, dataset.ApprovalsFile), []byte(approvals), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported %d approval examples to %s (%s plus the receipt export)\n",
				strings.Count(approvals, "\n"), out, dataset.ApprovalsFile)
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

func newDatasetVerifyCommand() *cobra.Command {
	var trustRoot string
	var allowPrefix, allowEmbeddedTrust bool
	cmd := &cobra.Command{
		Use:   "verify <export-dir>",
		Short: "Verify an approval dataset export offline",
		Long: "Verifies the receipt chain and decision records like 'agentctl receipts verify', then checks " +
			"that every example's receipt_seq and entry hash exist in the chain and that its label matches " +
			"the human decision receipt. A pass requires a signed export manifest that proves completeness " +
			"unless --allow-prefix is set. Pass --trust-root with a trust file pinned out of band. Without it the export's own " +
			"trust.json would decide who is trusted, so verification refuses unless --allow-embedded-trust is set.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if trustRoot == "" && !allowEmbeddedTrust {
				return fmt.Errorf("--trust-root is required: an export that carries its own trust root can be replaced with an attacker-signed copy and still pass. Pin a trust file out of band, or pass --allow-embedded-trust for a consistency-only check")
			}
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
			e, approvals, err := dataset.ReadDir(args[0])
			if err != nil {
				return err
			}
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
			if trustRoot == "" {
				fmt.Fprintln(stderr, "WARNING: trust root taken from the export itself (--allow-embedded-trust). This proves internal consistency only, not who signed it.")
			}
			rep, err := dataset.Verify(e, approvals, trusted)
			if receipts.IsManifestError(err) {
				fmt.Fprintf(stderr, "FAIL: manifest: %v\n", err)
				return errVerifyFailed
			}
			if err != nil {
				return err
			}
			c := rep.Receipts.Chain
			if c.OK {
				fmt.Fprintf(stdout, "PASS: %d receipts verified, head seq=%d\n", c.VerifiedCount, c.HeadSeq)
			} else {
				fmt.Fprintf(stderr, "FAIL: seq=%d reason=%s\n", c.FailedAtSeq, c.Reason)
			}
			for _, f := range rep.Receipts.RecordFailures {
				fmt.Fprintf(stderr, "FAIL: record seq=%d reason=%s\n", f.Seq, f.Reason)
			}
			for _, f := range rep.Failures {
				fmt.Fprintf(stderr, "FAIL: example line=%d seq=%d reason=%s\n", f.Line, f.Seq, f.Reason)
			}
			if len(rep.Failures) == 0 {
				fmt.Fprintf(stdout, "PASS: %d approval examples bound to their receipts\n", rep.ExamplesBound)
			}
			return completenessVerdict(stdout, stderr, rep.PrefixOK(), rep.Receipts.Complete(), allowPrefix)
		},
	}
	cmd.Flags().StringVar(&trustRoot, "trust-root", "", "trust file pinned out of band (receiptspec trust file format)")
	cmd.Flags().BoolVar(&allowPrefix, "allow-prefix", false, allowPrefixHelp)
	cmd.Flags().BoolVar(&allowEmbeddedTrust, "allow-embedded-trust", false, "use the export's own trust.json when --trust-root is not given; proves consistency only, not authenticity")
	return cmd
}
