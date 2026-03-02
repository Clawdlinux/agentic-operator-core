package billing

import (
	"math"
	"testing"
	"time"
)

func TestCostCalculator_BasicCalculation(t *testing.T) {
	calc := NewCostCalculator()

	event := &BillingEvent{
		TenantName:    "customer-alpha",
		WorkloadID:    "test-1",
		ModelTier:     "medium",
		InputTokens:   1000,
		OutputTokens:  2000,
		EstimatedCost: 0,
		CompletedAt:   time.Now(),
	}

	breakdown := calc.CalculateCost(event)

	// Medium tier: $0.01 per 1K tokens
	// 3000 tokens = 3 * $0.01 = $0.03
	// Operator fee (10%): $0.03 * 0.10 = $0.003
	// Subtotal: $0.033
	// Tax (8%): $0.033 * 0.08 = $0.00264
	// No discount
	// Final: $0.033 + $0.00264 = $0.03564

	expectedBase := 0.03
	if math.Abs(breakdown.BaseCost-expectedBase) > 0.0001 {
		t.Errorf("expected base cost %f, got %f", expectedBase, breakdown.BaseCost)
	}

	if breakdown.TotalTokens != 3000 {
		t.Errorf("expected 3000 total tokens, got %d", breakdown.TotalTokens)
	}

	if breakdown.FinalCost < 0.030 || breakdown.FinalCost > 0.040 {
		t.Errorf("expected final cost ~0.036, got %f", breakdown.FinalCost)
	}
}

func TestCostCalculator_CheapTier(t *testing.T) {
	calc := NewCostCalculator()

	event := &BillingEvent{
		TenantName:   "customer-alpha",
		ModelTier:    "cheap",
		InputTokens:  5000,
		OutputTokens: 5000,
	}

	breakdown := calc.CalculateCost(event)

	// Cheap tier: $0.001 per 1K tokens
	// 10000 tokens = 10 * $0.001 = $0.01
	expectedBase := 0.01
	if math.Abs(breakdown.BaseCost-expectedBase) > 0.0001 {
		t.Errorf("expected base %f, got %f", expectedBase, breakdown.BaseCost)
	}
}

func TestCostCalculator_ExpensiveTier(t *testing.T) {
	calc := NewCostCalculator()

	event := &BillingEvent{
		TenantName:   "customer-beta",
		ModelTier:    "expensive",
		InputTokens:  1000,
		OutputTokens: 1000,
	}

	breakdown := calc.CalculateCost(event)

	// Expensive tier: $0.03 per 1K tokens
	// 2000 tokens = 2 * $0.03 = $0.06
	expectedBase := 0.06
	if math.Abs(breakdown.BaseCost-expectedBase) > 0.0001 {
		t.Errorf("expected base %f, got %f", expectedBase, breakdown.BaseCost)
	}
}

func TestCostCalculator_WithDiscount(t *testing.T) {
	calc := NewCostCalculator()
	calc.SetDiscount("customer-alpha", 20.0) // 20% discount

	event := &BillingEvent{
		TenantName:   "customer-alpha",
		ModelTier:    "medium",
		InputTokens:  1000,
		OutputTokens: 2000,
	}

	breakdown := calc.CalculateCost(event)

	// Without discount: ~0.03564
	// With 20% discount on subtotal: should be ~20% less
	if breakdown.DiscountAmount <= 0 {
		t.Error("expected discount amount > 0")
	}

	// Final cost should be less than without discount
	calc2 := NewCostCalculator()
	breakdown2 := calc2.CalculateCost(event)

	if breakdown.FinalCost >= breakdown2.FinalCost {
		t.Errorf("discounted cost (%.4f) should be less than regular (%.4f)",
			breakdown.FinalCost, breakdown2.FinalCost)
	}
}

func TestCostCalculator_ZeroCost(t *testing.T) {
	calc := NewCostCalculator()

	event := &BillingEvent{
		TenantName:   "customer-test",
		ModelTier:    "cheap",
		InputTokens:  0,
		OutputTokens: 0,
	}

	breakdown := calc.CalculateCost(event)

	if breakdown.FinalCost != 0 {
		t.Errorf("expected zero cost for zero tokens, got %f", breakdown.FinalCost)
	}
}

func TestCostCalculator_MonthlyAggregation(t *testing.T) {
	calc := NewCostCalculator()

	// Simulate 3 workloads in a month
	events := []*BillingEvent{
		{
			TenantName:   "customer-alpha",
			ModelTier:    "cheap",
			InputTokens:  1000,
			OutputTokens: 1000,
		},
		{
			TenantName:   "customer-alpha",
			ModelTier:    "medium",
			InputTokens:  2000,
			OutputTokens: 2000,
		},
		{
			TenantName:   "customer-alpha",
			ModelTier:    "expensive",
			InputTokens:  500,
			OutputTokens: 500,
		},
	}

	total := calc.CalculateMonthlyTotal(events, "customer-alpha")

	if total.TotalTokens != 7000 {
		t.Errorf("expected 7000 total tokens, got %d", total.TotalTokens)
	}

	if total.FinalCost <= 0 {
		t.Error("expected positive monthly cost")
	}

	// Monthly cost should equal sum of individual costs
	individualSum := 0.0
	for _, event := range events {
		breakdown := calc.CalculateCost(event)
		individualSum += breakdown.FinalCost
	}

	if math.Abs(total.FinalCost-individualSum) > 0.001 {
		t.Errorf("monthly total %.4f should equal sum of events %.4f",
			total.FinalCost, individualSum)
	}
}

func TestCostCalculator_AnnualContract(t *testing.T) {
	calc := NewCostCalculator()

	// Annual contract = 30% discount
	calc.SetDiscount("customer-enterprise", 30.0)

	event := &BillingEvent{
		TenantName:   "customer-enterprise",
		ModelTier:    "expensive",
		InputTokens:  10000,
		OutputTokens: 10000,
	}

	breakdown := calc.CalculateCost(event)

	// Cost should be ~30% cheaper
	if breakdown.DiscountAmount <= 0 {
		t.Error("expected discount for annual contract")
	}

	if breakdown.FinalCost <= 0 {
		t.Error("expected positive cost after discount")
	}
}

func TestCostCalculator_CustomPricing(t *testing.T) {
	calc := NewCostCalculator()
	calc.SetModelPrice("custom", 0.005) // $0.005 per 1K tokens

	event := &BillingEvent{
		TenantName:   "customer-custom",
		ModelTier:    "custom",
		InputTokens:  1000,
		OutputTokens: 1000,
	}

	breakdown := calc.CalculateCost(event)

	// 2000 tokens * $0.005/1K = $0.01
	expectedBase := 0.01
	if math.Abs(breakdown.BaseCost-expectedBase) > 0.0001 {
		t.Errorf("expected base %f, got %f", expectedBase, breakdown.BaseCost)
	}
}

func TestCostCalculator_UnknownTierFallback(t *testing.T) {
	calc := NewCostCalculator()

	event := &BillingEvent{
		TenantName:   "customer-test",
		ModelTier:    "unknown-tier",
		InputTokens:  1000,
		OutputTokens: 1000,
	}

	breakdown := calc.CalculateCost(event)

	// Should fallback to medium tier pricing
	expectedBase := 0.02 // medium: $0.01 per 1K, 2000 tokens
	if math.Abs(breakdown.BaseCost-expectedBase) > 0.0001 {
		t.Errorf("expected fallback to medium (%.4f), got %.4f", expectedBase, breakdown.BaseCost)
	}
}

func TestCostCalculator_NegativeCostCapped(t *testing.T) {
	calc := NewCostCalculator()

	// Try to set 150% discount (should be capped)
	calc.SetDiscount("customer-test", 150.0)

	event := &BillingEvent{
		TenantName:   "customer-test",
		ModelTier:    "cheap",
		InputTokens:  100,
		OutputTokens: 100,
	}

	breakdown := calc.CalculateCost(event)

	// Cost should never be negative
	if breakdown.FinalCost < 0 {
		t.Errorf("expected non-negative cost, got %f", breakdown.FinalCost)
	}
}
