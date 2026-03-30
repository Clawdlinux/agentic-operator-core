package billing

// OSS-PRIVATE-ALLOW: Billing type models are temporarily kept during staged private migration.

import "time"

// BillingEvent records a completed workload and its cost.
type BillingEvent struct {
	TenantName    string
	WorkloadID    string
	ModelUsed     string // provider/model
	TaskCategory  string // validation, analysis, reasoning
	InputTokens   int
	OutputTokens  int
	ModelTier     string // cheap, medium, expensive
	EstimatedCost float64
	CompletedAt   time.Time
}

// Invoice represents a monthly bill for a tenant.
type Invoice struct {
	ID              string
	TenantName      string
	StripeInvoiceID string
	PeriodStart     time.Time
	PeriodEnd       time.Time
	Items           []InvoiceLineItem
	Subtotal        float64
	TaxPercent      float64
	TaxAmount       float64
	DiscountPercent float64
	DiscountAmount  float64
	Total           float64
	Status          string // draft, sent, paid, failed
	CreatedAt       time.Time
	PaidAt          *time.Time
}

// InvoiceLineItem is a line on an invoice.
type InvoiceLineItem struct {
	Description string
	Quantity    int64
	UnitPrice   float64
	Amount      float64
}

// BillingAccount represents a tenant's billing setup.
type BillingAccount struct {
	TenantName       string
	StripeCustomerID string
	BillingEmail     string
	AnnualContract   bool
	DiscountPercent  float64 // 0-100
	IsActive         bool
	CreatedAt        time.Time
}

// CostBreakdown shows cost calculation details.
type CostBreakdown struct {
	InputTokens       int
	OutputTokens      int
	TotalTokens       int
	BaseCost          float64
	OperatorMarkup    float64 // 10% of base
	SubtotalBeforeTax float64
	Tax               float64
	DiscountAmount    float64
	FinalCost         float64
}
