/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/baseline"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/eval"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/learned"
)

func newDecisionCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decision",
		Short: "Evaluate the decision model offline",
		// Offline only. Do not load a kubeconfig.
		PersistentPreRunE: func(*cobra.Command, []string) error { return opts.validateOutput() },
	}
	cmd.AddCommand(newDecisionEvalCommand(opts))
	return cmd
}

func newDecisionEvalCommand(opts *cliOptions) *cobra.Command {
	var data, model, trustRoot string
	var withBaseline, allowUnverified bool
	var threshold int64
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Score a model artifact against the approval dataset",
		Long: "Verifies the dataset like 'agentctl dataset verify', then reports how many human rejects and edits " +
			"the model would have escalated (recall), how many approvals it would have sent to review, precision, " +
			"a calibration table, and ECE. --dataset is an export dir or a bare approvals.jsonl. Unverified data " +
			"is refused unless --allow-unverified.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var override *int64
			if cmd.Flags().Changed("threshold-micro") {
				if threshold < 0 || threshold > decision.MicroScale {
					return fmt.Errorf("--threshold-micro must be 0 to %d", decision.MicroScale)
				}
				override = &threshold
			}
			exs, verified, err := loadEvalDataset(data, trustRoot)
			if err != nil {
				return err
			}
			if !verified && !allowUnverified {
				return errors.New("dataset is not verified against signed receipts; rerun with --allow-unverified to evaluate anyway")
			}
			if len(exs) == 0 {
				return errors.New("dataset has no examples")
			}
			m, err := learned.Load(model)
			if err != nil {
				return err
			}
			var rep eval.Report
			rep.Counts(exs, verified)
			if rep.Model, err = eval.Evaluate(cmd.Context(), exs, m, override); err != nil {
				return err
			}
			if withBaseline {
				b, err := eval.Evaluate(cmd.Context(), exs, baseline.Scorer{}, nil)
				if err != nil {
					return err
				}
				rep.Baseline = &b
			}
			if opts.Output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			printEvalReport(cmd.OutOrStdout(), rep)
			return nil
		},
	}
	cmd.Flags().StringVar(&data, "dataset", "", "export dir (receipts plus approvals.jsonl) or an approvals.jsonl file")
	cmd.Flags().StringVar(&model, "model", "", "model artifact JSON")
	cmd.Flags().BoolVar(&withBaseline, "baseline", false, "also score the hand-weighted baseline and compare")
	cmd.Flags().Int64Var(&threshold, "threshold-micro", 0, "escalation threshold in micro-units, overrides the artifact")
	cmd.Flags().BoolVar(&allowUnverified, "allow-unverified", false, "evaluate data that does not verify against receipts, such as synthetic rows")
	cmd.Flags().StringVar(&trustRoot, "trust-root", "", "trust file pinned out of band (receiptspec trust file format)")
	for _, f := range []string{"dataset", "model"} {
		_ = cmd.MarkFlagRequired(f)
	}
	return cmd
}

// loadEvalDataset reads examples. Only an export dir whose receipts and
// examples all verify counts as verified.
func loadEvalDataset(path, trustRoot string) ([]dataset.Example, bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}
	if !st.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, err
		}
		exs, err := eval.ParseJSONL(data)
		return exs, false, err
	}
	e, approvals, err := dataset.ReadDir(path)
	if err != nil {
		data, rerr := os.ReadFile(filepath.Join(path, dataset.ApprovalsFile))
		if rerr != nil {
			return nil, false, err
		}
		exs, perr := eval.ParseJSONL(data)
		return exs, false, perr
	}
	var trusted []receiptspec.TrustedKey
	if trustRoot != "" {
		data, err := os.ReadFile(trustRoot)
		if err != nil {
			return nil, false, err
		}
		if trusted, err = receiptspec.LoadTrustedKeys(data); err != nil {
			return nil, false, err
		}
	}
	rep, verr := dataset.Verify(e, approvals, trusted)
	exs, err := eval.ParseJSONL([]byte(approvals))
	if err != nil {
		return nil, false, err
	}
	return exs, verr == nil && rep.OK(), nil
}

func micro(v int64) string {
	return fmt.Sprintf("%d.%06d", v/decision.MicroScale, v%decision.MicroScale)
}

func printEvalReport(w io.Writer, r eval.Report) {
	if len(r.Warnings) > 0 {
		_, _ = fmt.Fprintln(w, "WARNING: these numbers are not evidence of production quality.")
		for _, s := range r.Warnings {
			_, _ = fmt.Fprintln(w, "WARNING: "+s)
		}
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintf(w, "examples %d (human %d, synthetic %d, verified %v)\n\n", r.Examples, r.Human, r.Synthetic, r.Verified)
	results := []eval.Result{r.Model}
	if r.Baseline != nil {
		results = append(results, *r.Baseline)
	}
	for _, res := range results {
		_, _ = fmt.Fprintf(w, "%s %s threshold %s\n", res.ModelID, res.Version, micro(res.ThresholdMicro))
		_, _ = fmt.Fprintf(w, "  humans rejected or edited %d, model escalated %d (recall %s)\n", res.HumanRejected, res.Caught, micro(res.RecallMicro))
		_, _ = fmt.Fprintf(w, "  humans approved %d, model escalated %d (extra review %s)\n", res.HumanApproved, res.ExtraReview, micro(res.ExtraReviewMicro))
		_, _ = fmt.Fprintf(w, "  precision %s, ECE %s\n", micro(res.PrecisionMicro), micro(res.ECEMicro))
		_, _ = fmt.Fprintln(w, "  bin              count  expected  observed")
		for _, b := range res.Bins {
			_, _ = fmt.Fprintf(w, "  %s-%s %6d  %s  %s\n", micro(b.LowerMicro)[:4], micro(b.UpperMicro)[:4], b.Count, micro(b.MeanPredictedMicro), micro(b.ObservedMicro))
		}
		_, _ = fmt.Fprintln(w)
	}
	if r.Baseline != nil {
		m, b := r.Model, *r.Baseline
		_, _ = fmt.Fprintf(w, "model vs baseline: recall %s vs %s, extra review %s vs %s, precision %s vs %s, ECE %s vs %s\n",
			micro(m.RecallMicro), micro(b.RecallMicro), micro(m.ExtraReviewMicro), micro(b.ExtraReviewMicro),
			micro(m.PrecisionMicro), micro(b.PrecisionMicro), micro(m.ECEMicro), micro(b.ECEMicro))
	}
}
