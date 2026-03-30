package billing

// OSS-PRIVATE-ALLOW: Billing package retained temporarily while private extraction is in progress.

import "sync"

// CostCalculator computes costs for workloads and invoices.
type CostCalculator struct {
	mu sync.RWMutex

	// modelPricing: tier → cost per 1K tokens
	modelPricing map[string]float64

	// operatorFee: markup percentage (e.g., 0.10 = 10%)
	operatorFee float64

	// taxRate: sales tax percentage (e.g., 0.08 = 8%)
	taxRate float64

	// discounts: tenant → discount percentage (0-100)
	discounts map[string]float64
}

// NewCostCalculator creates a calculator with default pricing.
func NewCostCalculator() *CostCalculator {
	return &CostCalculator{
		modelPricing: map[string]float64{
			"cheap":     0.001, // GPT-3.5 equivalent: $0.001 / 1K tokens
			"medium":    0.010, // GPT-4 equivalent: $0.01 / 1K tokens
			"expensive": 0.030, // GPT-4 Turbo equivalent: $0.03 / 1K tokens
		},
		operatorFee: 0.10, // 10% operator fee
		taxRate:     0.08, // 8% sales tax (US average)
		discounts:   make(map[string]float64),
	}
}

// SetModelPrice sets the price per 1K tokens for a tier.
func (cc *CostCalculator) SetModelPrice(tier string, pricePerK float64) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.modelPricing[tier] = pricePerK
}

// SetDiscount sets the discount for a tenant.
func (cc *CostCalculator) SetDiscount(tenantName string, discountPercent float64) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.discounts[tenantName] = discountPercent
}

// CalculateCost computes the final cost for a billing event.
func (cc *CostCalculator) CalculateCost(event *BillingEvent) *CostBreakdown {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	breakdown := &CostBreakdown{
		InputTokens:  event.InputTokens,
		OutputTokens: event.OutputTokens,
		TotalTokens:  event.InputTokens + event.OutputTokens,
	}

	// Get price per 1K tokens for the model tier
	pricePerK, ok := cc.modelPricing[event.ModelTier]
	if !ok {
		pricePerK = cc.modelPricing["medium"] // fallback to medium
	}

	// Calculate base cost
	breakdown.BaseCost = (float64(breakdown.TotalTokens) / 1000.0) * pricePerK

	// Add operator markup (10% of base)
	breakdown.OperatorMarkup = breakdown.BaseCost * cc.operatorFee

	// Subtotal before tax
	breakdown.SubtotalBeforeTax = breakdown.BaseCost + breakdown.OperatorMarkup

	// Add tax
	breakdown.Tax = breakdown.SubtotalBeforeTax * cc.taxRate

	// Apply tenant discount
	discount := cc.discounts[event.TenantName]
	if discount > 100 {
		discount = 100 // cap at 100%
	}
	breakdown.DiscountAmount = breakdown.SubtotalBeforeTax * (discount / 100.0)

	// Final cost: subtotal + tax - discount
	breakdown.FinalCost = (breakdown.SubtotalBeforeTax + breakdown.Tax) - breakdown.DiscountAmount

	// Ensure cost is never negative
	if breakdown.FinalCost < 0 {
		breakdown.FinalCost = 0
	}

	return breakdown
}

// CalculateMonthlyTotal sums costs for multiple events (monthly invoice).
func (cc *CostCalculator) CalculateMonthlyTotal(events []*BillingEvent, tenantName string) *CostBreakdown {
	total := &CostBreakdown{
		TotalTokens: 0,
	}

	// Aggregate all events
	for _, event := range events {
		breakdown := cc.CalculateCost(event)
		total.InputTokens += breakdown.InputTokens
		total.OutputTokens += breakdown.OutputTokens
		total.TotalTokens += breakdown.TotalTokens
		total.BaseCost += breakdown.BaseCost
		total.OperatorMarkup += breakdown.OperatorMarkup
		total.SubtotalBeforeTax += breakdown.SubtotalBeforeTax
		total.Tax += breakdown.Tax
		total.DiscountAmount += breakdown.DiscountAmount
		total.FinalCost += breakdown.FinalCost
	}

	return total
}

// GetPricing returns current pricing (for billing dashboard).
func (cc *CostCalculator) GetPricing() map[string]float64 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	pricing := make(map[string]float64)
	for tier, price := range cc.modelPricing {
		pricing[tier] = price
	}
	return pricing
}

// GetTenantDiscount returns discount for a tenant.
func (cc *CostCalculator) GetTenantDiscount(tenantName string) float64 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.discounts[tenantName]
}
