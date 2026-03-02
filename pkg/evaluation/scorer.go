package evaluation

import (
	"context"
	"strings"
	"unicode"
)

// QualityScorer provides heuristic-based quality scoring for agent outputs.
// Requires no ML — uses keyword analysis, length heuristics, and structural checks.
type QualityScorer struct{}

// NewQualityScorer creates a new QualityScorer.
func NewQualityScorer() *QualityScorer { return &QualityScorer{} }

// Score evaluates the quality of an agent execution record.
func (s *QualityScorer) Score(_ context.Context, record ExecutionRecord) QualityEvaluation {
	eval := QualityEvaluation{Details: make(map[string]interface{})}

	eval.Relevance = s.scoreRelevance(record.Output, record.TaskCategory)
	eval.HallucinRisk = s.detectHallucinationRisk(record.Output)
	eval.Completeness = s.scoreCompleteness(record.Output, record.TaskCategory)
	eval.Clarity = s.scoreClarity(record.Output)

	// Weighted average: relevance 35%, anti-hallucination 25%, completeness 20%, clarity 20%
	eval.OverallScore = clamp(int(
		float64(eval.Relevance)*0.35+
			float64(100-eval.HallucinRisk)*0.25+
			float64(eval.Completeness)*0.20+
			float64(eval.Clarity)*0.20), 0, 100)

	eval.Details["relevance"] = eval.Relevance
	eval.Details["hallucination_risk"] = eval.HallucinRisk
	eval.Details["completeness"] = eval.Completeness
	eval.Details["clarity"] = eval.Clarity

	return eval
}

// scoreRelevance checks whether the output contains keywords appropriate to the task category.
func (s *QualityScorer) scoreRelevance(output, category string) int {
	if output == "" {
		return 0
	}
	lower := strings.ToLower(output)
	score := 60

	switch category {
	case "validation":
		for _, kw := range []string{"valid", "invalid", "pass", "fail", "yes", "no", "correct", "incorrect", "true", "false"} {
			if strings.Contains(lower, kw) {
				score += 10
				break
			}
		}
	case "analysis":
		keywords := []string{"trend", "pattern", "insight", "analysis", "data", "result", "finding", "shows", "indicates", "suggests"}
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score += 4
			}
		}
	case "reasoning":
		keywords := []string{"therefore", "because", "conclude", "reason", "implies", "suggests", "evidence", "thus", "hence"}
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score += 4
			}
		}
	default:
		score = 70
	}

	return clamp(score, 0, 100)
}

// detectHallucinationRisk checks for patterns that indicate contradictions or fabrications.
func (s *QualityScorer) detectHallucinationRisk(output string) int {
	if output == "" {
		return 50 // empty output is uncertain
	}
	risk := 0
	lower := strings.ToLower(output)

	// Contradictory opening + confident body = hallucination signal
	contradictions := []string{
		"i don't have access",
		"i cannot verify",
		"i don't have information",
		"as of my knowledge cutoff",
		"i'm not able to access",
		"i cannot confirm",
	}
	for _, c := range contradictions {
		if strings.Contains(lower, c) {
			risk += 15
		}
	}

	// Extremely confident language in long response = risky
	if len(output) > 1000 && strings.Count(lower, "definitely") > 2 {
		risk += 20
	}

	// Too short = didn't really answer
	if len(output) < 20 {
		risk += 30
	}

	return clamp(risk, 0, 100)
}

// scoreCompleteness checks whether the response length is appropriate for the task type.
func (s *QualityScorer) scoreCompleteness(output, category string) int {
	length := len(output)
	if length == 0 {
		return 0
	}
	switch category {
	case "validation":
		if length < 10 {
			return 20
		}
		if length > 500 {
			return 65 // verbose for a yes/no
		}
		return 85
	case "analysis":
		if length < 100 {
			return 40
		}
		if length < 300 {
			return 70
		}
		return 90
	case "reasoning":
		if length < 200 {
			return 50
		}
		return 85
	}
	if length < 50 {
		return 50
	}
	return 75
}

// scoreClarity evaluates structural quality of the response.
func (s *QualityScorer) scoreClarity(output string) int {
	if output == "" {
		return 0
	}
	score := 70

	// Reward multi-line structured output
	if strings.Contains(output, "\n") {
		score += 5
	}
	if strings.Count(output, "-") > 2 || strings.Count(output, "•") > 1 || strings.Count(output, "1.") > 0 {
		score += 10
	}

	// Penalise ALL-CAPS shouting
	upper := 0
	total := 0
	for _, r := range output {
		if unicode.IsLetter(r) {
			total++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if total > 0 && float64(upper)/float64(total) > 0.4 {
		score -= 15
	}

	return clamp(score, 0, 100)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
