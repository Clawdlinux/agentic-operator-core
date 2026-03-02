package routing

import (
	"strings"
	"unicode/utf8"
)

// TaskCategory represents the classification of a task for cost-aware model routing
type TaskCategory string

const (
	// CategoryValidation - Simple validation, formatting, parsing tasks
	// Routed to: Phi-3.5 (cheapest: $0.01/1K tokens)
	CategoryValidation TaskCategory = "validation"

	// CategoryAnalysis - Analysis, synthesis, extraction tasks
	// Routed to: Mixtral ($0.27/1K tokens)
	CategoryAnalysis TaskCategory = "analysis"

	// CategoryReasoning - Complex reasoning, novel problems, consensus voting
	// Routed to: Claude ($0.735/1K tokens, most expensive)
	CategoryReasoning TaskCategory = "reasoning"
)

// TaskClassifier classifies prompts to determine optimal model routing
type TaskClassifier struct {
	// Threshold for prompt length (chars)
	// < ValidationThreshold = likely validation
	// ValidationThreshold - AnalysisThreshold = likely analysis
	// > AnalysisThreshold = likely reasoning
	ValidationThreshold int
	AnalysisThreshold   int

	// Keywords that indicate each category
	ValidationKeywords []string
	AnalysisKeywords   []string
	ReasoningKeywords  []string
}

// NewDefaultClassifier returns a classifier with default heuristics
func NewDefaultClassifier() *TaskClassifier {
	return &TaskClassifier{
		ValidationThreshold: 200,   // Short prompts
		AnalysisThreshold:   800,   // Medium prompts
		ValidationKeywords: []string{
			"verify", "check", "validate", "parse", "format",
			"extract", "split", "join", "convert", "normalize",
			"match", "search", "find",
		},
		AnalysisKeywords: []string{
			"analyze", "summarize", "summarise", "synthesis",
			"extract", "identify", "classify", "categorize",
			"compare", "contrast", "evaluate", "assess",
			"break", "decompose", "explain",
		},
		ReasoningKeywords: []string{
			"reason", "think", "decide", "determine",
			"novel", "unique", "infer", "deduce",
			"novel", "creative", "generate", "design",
			"complex", "elaborate", "discuss",
		},
	}
}

// Classify analyzes a prompt and returns the recommended task category
func (c *TaskClassifier) Classify(prompt string) TaskCategory {
	if prompt == "" {
		return CategoryReasoning // Default to expensive for safety
	}

	// Normalize prompt for keyword matching
	normalized := strings.ToLower(prompt)
	wordCount := len(strings.Fields(prompt))
	charCount := utf8.RuneCountInString(prompt)

	// Score each category
	validationScore := c.scoreValidation(normalized, wordCount, charCount)
	analysisScore := c.scoreAnalysis(normalized, wordCount, charCount)
	reasoningScore := c.scoreReasoning(normalized, wordCount, charCount)

	// Return highest scoring category
	// For classification: higher score wins
	// On tie, prefer more expensive model for safety (reasoning > analysis > validation)
	maxScore := validationScore
	result := CategoryValidation

	if analysisScore > maxScore {
		maxScore = analysisScore
		result = CategoryAnalysis
	}

	if reasoningScore > maxScore {
		maxScore = reasoningScore
		result = CategoryReasoning
	}

	// If no clear signal, default to reasoning for safety
	if maxScore == 0 {
		return CategoryReasoning
	}

	return result
}

// scoreValidation returns a score (0-100) for validation category
func (c *TaskClassifier) scoreValidation(normalized string, wordCount, charCount int) int {
	score := 0

	// Check for validation keywords (high weight - most important)
	keywordMatches := 0
	for _, keyword := range c.ValidationKeywords {
		if strings.Contains(normalized, keyword) {
			keywordMatches++
		}
	}
	score += (keywordMatches * 25) // Each keyword match adds 25 points

	// Short prompts are likely validation (lower weight)
	if charCount < c.ValidationThreshold {
		score += 10
	}
	if wordCount < 20 {
		score += 10
	}

	return score
}

// scoreAnalysis returns a score (0-100) for analysis category
func (c *TaskClassifier) scoreAnalysis(normalized string, wordCount, charCount int) int {
	score := 0

	// Check for analysis keywords (high weight - most important)
	keywordMatches := 0
	for _, keyword := range c.AnalysisKeywords {
		if strings.Contains(normalized, keyword) {
			keywordMatches++
		}
	}
	score += (keywordMatches * 25) // Each keyword match adds 25 points

	// Medium-length prompts (lower weight)
	if charCount >= c.ValidationThreshold && charCount < c.AnalysisThreshold {
		score += 15
	}
	if wordCount >= 20 && wordCount < 200 {
		score += 15
	}

	return score
}

// scoreReasoning returns a score (0-100) for reasoning category
func (c *TaskClassifier) scoreReasoning(normalized string, wordCount, charCount int) int {
	score := 0

	// Check for reasoning keywords (high weight - most important)
	keywordMatches := 0
	for _, keyword := range c.ReasoningKeywords {
		if strings.Contains(normalized, keyword) {
			keywordMatches++
		}
	}
	score += (keywordMatches * 25) // Each keyword match adds 25 points

	// Complex markers
	if strings.Contains(normalized, "why") || strings.Contains(normalized, "how") {
		score += 20
	}
	if strings.Contains(normalized, "?") {
		// Question marks indicate more complex inquiry
		qmarkCount := strings.Count(normalized, "?")
		score += (qmarkCount * 8)
	}

	// Long prompts are likely reasoning (lower weight)
	if charCount >= c.AnalysisThreshold {
		score += 10
	}
	if wordCount >= 100 {
		score += 10
	}

	return score
}
