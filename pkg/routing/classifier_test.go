package routing

import (
	"testing"
)

func TestClassifyValidation(t *testing.T) {
	classifier := NewDefaultClassifier()

	testCases := []struct {
		name     string
		prompt   string
		expected TaskCategory
	}{
		{
			name:     "Short parse request",
			prompt:   "Parse this JSON: {\"key\": \"value\"}",
			expected: CategoryValidation,
		},
		{
			name:     "Format conversion",
			prompt:   "Convert this date to ISO format: 2026-03-02",
			expected: CategoryValidation,
		},
		{
			name:     "Simple verification",
			prompt:   "Verify this email: test@example.com",
			expected: CategoryValidation,
		},
		{
			name:     "Extract data",
			prompt:   "Extract the price from: Item costs $99.99",
			expected: CategoryValidation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.prompt)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s for prompt: %s", tc.expected, result, tc.prompt)
			}
		})
	}
}

func TestClassifyAnalysis(t *testing.T) {
	classifier := NewDefaultClassifier()

	testCases := []struct {
		name     string
		prompt   string
		expected TaskCategory
	}{
		{
			name: "Analyze market trends",
			prompt: `Analyze the recent market trends in tech stocks. 
			Look at the performance of AAPL, MSFT, and GOOGL over the last quarter.
			What patterns do you see? Summarize your findings.`,
			expected: CategoryAnalysis,
		},
		{
			name: "Compare implementations",
			prompt: `Compare these two sorting algorithms in terms of time complexity, 
			space complexity, and practical performance. Contrast their strengths and weaknesses.`,
			expected: CategoryAnalysis,
		},
		{
			name: "Classify data",
			prompt: `Categorize these customer reviews as positive, negative, or neutral. 
			Identify the main themes in each category.`,
			expected: CategoryAnalysis,
		},
		{
			name: "Evaluate options",
			prompt: `Assess these three architectural approaches for a microservices system.
			What are the tradeoffs? Which would you recommend for a startup?`,
			expected: CategoryAnalysis,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.prompt)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s for prompt: %s", tc.expected, result, tc.prompt)
			}
		})
	}
}

func TestClassifyReasoning(t *testing.T) {
	classifier := NewDefaultClassifier()

	testCases := []struct {
		name     string
		prompt   string
		expected TaskCategory
	}{
		{
			name: "Novel problem solving",
			prompt: `We have a distributed system with eventual consistency. 
			How do we ensure critical operations maintain strong consistency without sacrificing scalability?
			Think through the tradeoffs and design a novel solution.
			Explain your reasoning and why this approach is better than existing patterns.`,
			expected: CategoryReasoning,
		},
		{
			name: "Complex decision",
			prompt: `Why did the Apollo program succeed while other moon missions failed?
			What were the key factors? How do those lessons apply to modern space exploration?
			Consider technical, organizational, and historical perspectives.`,
			expected: CategoryReasoning,
		},
		{
			name: "Creative generation",
			prompt: `Design a unique business model for an AI coding assistant that prioritizes developer autonomy.
			What are the ethical implications? How would this differ from current approaches?
			Think creatively about novel revenue streams and customer value.`,
			expected: CategoryReasoning,
		},
		{
			name: "Elaborate discussion",
			prompt: `Discuss the implications of AGI on society. Consider economic disruption, governance challenges,
			and opportunities for human flourishing. What should we be building today to prepare?
			Elaborate on 3-4 key scenarios and how they interconnect.`,
			expected: CategoryReasoning,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.prompt)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s for prompt: %s", tc.expected, result, tc.prompt)
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	classifier := NewDefaultClassifier()

	testCases := []struct {
		name     string
		prompt   string
		minScore TaskCategory // Minimum acceptable (for ambiguous cases)
	}{
		{
			name:     "Empty prompt",
			prompt:   "",
			minScore: CategoryReasoning, // Default to expensive for safety
		},
		{
			name:     "Single word",
			prompt:   "analyze",
			minScore: CategoryValidation, // Can be any, but should not crash
		},
		{
			name:     "Ambiguous short",
			prompt:   "What?",
			minScore: CategoryValidation,
		},
		{
			name:     "Multiple keywords mixed",
			prompt:   "Verify and analyze the data, then reason about what we learned",
			minScore: CategoryAnalysis, // Has mix, should lean toward analysis or reasoning
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.prompt)
			// For edge cases, we just verify it returns a valid category without crashing
			if result != CategoryValidation && result != CategoryAnalysis && result != CategoryReasoning {
				t.Errorf("Expected valid category, got: %s", result)
			}
		})
	}
}

func TestScoreFunctions(t *testing.T) {
	classifier := NewDefaultClassifier()

	// Validation should score high for validation keywords
	short := "Parse JSON and format the data"
	validScore := classifier.scoreValidation(short, 5, 30)
	if validScore < 25 {
		t.Errorf("Validation prompt should score >25 for validation, got %d", validScore)
	}

	// Analysis should score high for analysis keywords
	medium := "Analyze the market trends and identify the key patterns"
	analysisScore := classifier.scoreAnalysis(medium, 9, 55)
	if analysisScore < 25 {
		t.Errorf("Analysis prompt should score >25 for analysis, got %d", analysisScore)
	}

	// Reasoning should score high for reasoning keywords
	long := "Why did this system fail? Think deeply and reason about the implications"
	reasoningScore := classifier.scoreReasoning(long, 13, 75)
	if reasoningScore < 25 {
		t.Errorf("Reasoning prompt should score >25 for reasoning, got %d", reasoningScore)
	}
}

func TestKeywordMatching(t *testing.T) {
	classifier := NewDefaultClassifier()

	// Test validation keywords
	validationPrompts := []string{
		"verify the data",
		"check this format",
		"parse the JSON",
		"validate the email",
		"normalize the input",
	}

	for _, prompt := range validationPrompts {
		result := classifier.Classify(prompt)
		if result != CategoryValidation {
			t.Errorf("Expected validation for '%s', got %s", prompt, result)
		}
	}

	// Test analysis keywords
	analysisPrompts := []string{
		"analyze the trends over the last quarter",
		"summarize the key findings from the report and identify the main themes",
		"classify these items into categories based on their characteristics",
		"compare the two approaches in terms of efficiency",
		"evaluate the options and recommend the best one",
	}

	for _, prompt := range analysisPrompts {
		result := classifier.Classify(prompt)
		if result != CategoryAnalysis {
			t.Errorf("Expected analysis for '%s', got %s", prompt, result)
		}
	}

	// Test reasoning keywords
	reasoningPrompts := []string{
		"reason about why this happened",
		"think deeply about the implications",
		"design a novel solution to this problem",
		"elaborate on your previous point",
		"infer what will happen next",
	}

	for _, prompt := range reasoningPrompts {
		result := classifier.Classify(prompt)
		if result != CategoryReasoning {
			t.Errorf("Expected reasoning for '%s', got %s", prompt, result)
		}
	}
}

func TestClassifierConsistency(t *testing.T) {
	classifier := NewDefaultClassifier()

	// Same prompt should always give same result
	prompt := "Analyze the market data and identify trends"
	result1 := classifier.Classify(prompt)
	result2 := classifier.Classify(prompt)
	result3 := classifier.Classify(prompt)

	if result1 != result2 || result2 != result3 {
		t.Errorf("Classifier should be consistent: got %s, %s, %s", result1, result2, result3)
	}
}
