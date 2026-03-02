# Phase 9: Billing Integration

**Objective:** Complete the SaaS loop: track usage, calculate costs, invoice customers, process payments.

**Timeline:** 1-2 hours | **Deployment:** agentic-prod cluster

---

## Architecture

```
┌─────────────────────────────────────────────┐
│ Auto-Scaler (Phase 8)                       │
├─────────────────────────────────────────────┤
│ Records scaling decisions + model changes   │
└────────────┬────────────────────────────────┘
             │ (triggers on completion)
             ↓
┌─────────────────────────────────────────────┐
│ Billing Engine (Phase 9 NEW)                │
├─────────────────────────────────────────────┤
│ 1. Track workload completion                │
│ 2. Calculate cost:                          │
│    - LLM API charges (per token)            │
│    - Model tier markup (cheap/medium/exp)   │
│    - Operator fee (10% of LLM cost)         │
│    - Discount (if annual contract)          │
│ 3. Aggregate by tenant per month            │
│ 4. Generate invoice (PDF)                   │
│ 5. Send to Stripe for payment               │
└────────────┬────────────────────────────────┘
             │
             ↓
┌─────────────────────────────────────────────┐
│ Stripe Integration                          │
├─────────────────────────────────────────────┤
│ - Customer management (tenant ↔ Stripe ID)  │
│ - Invoice creation + delivery               │
│ - Payment tracking (received/pending/failed)│
│ - Webhook handling (payment confirmed)      │
└────────────┬────────────────────────────────┘
             │
             ↓
┌─────────────────────────────────────────────┐
│ Customer Portal (Billing Dashboard)         │
├─────────────────────────────────────────────┤
│ - Real-time cost breakdown                  │
│ - Usage graphs (tokens, model tiers)        │
│ - Invoice history + PDF download            │
│ - Payment method management                 │
│ - Cost alerts (threshold notifications)     │
└─────────────────────────────────────────────┘
```

---

## Implementation

### Step 1: Billing Types

**File:** `pkg/billing/types.go`

```go
type BillingEvent struct {
	TenantName      string
	WorkloadID      string
	CompletedAt     time.Time
	InputTokens     int
	OutputTokens    int
	ModelTier       string // cheap, medium, expensive
	EstimatedCost   float64
	ActualCost      float64 // After processing through Stripe
}

type Invoice struct {
	ID              string
	TenantName      string
	StripeInvoiceID string
	PeriodStart     time.Time
	PeriodEnd       time.Time
	Items           []InvoiceItem
	Subtotal        float64
	Tax             float64 // Sales tax where applicable
	Total           float64
	Status          string // draft, sent, paid, failed
	CreatedAt       time.Time
	PaidAt          *time.Time
}

type InvoiceItem struct {
	Description     string
	Quantity        int64 // tokens or hours
	UnitPrice       float64
	Amount          float64
}

type BillingAccount struct {
	TenantName      string
	StripeCustomerID string
	PaymentMethod   string // card last 4 digits
	BillingEmail    string
	AnnualContract  bool
	DiscountPercent float64
	IsActive        bool
}
```

### Step 2: Cost Calculator

**File:** `pkg/billing/calculator.go`

```go
type CostCalculator struct {
	modelPricing map[string]float64 // per 1K tokens
	operatorFee  float64             // 10% markup
	discount     map[string]float64  // tenant discount %
}

func (cc *CostCalculator) CalculateCost(event *BillingEvent) float64 {
	baseCost := cc.calculateTokenCost(event)
	operatorCost := baseCost * cc.operatorFee
	totalBeforeDiscount := baseCost + operatorCost
	
	discount := cc.discount[event.TenantName]
	finalCost := totalBeforeDiscount * (1 - discount)
	
	return finalCost
}
```

### Step 3: Invoice Generator

**File:** `pkg/billing/invoicer.go`

```go
type Invoicer struct {
	db *sql.DB
	stripe *stripe.Client
}

func (inv *Invoicer) GenerateMonthlyInvoice(tenantName string, month time.Time) (*Invoice, error) {
	// 1. Query all billing events for tenant + month
	// 2. Group by model tier
	// 3. Calculate subtotal, tax, discount
	// 4. Create Invoice object
	// 5. Upload to Stripe
	// 6. Send email to customer
	// 7. Store in DB
}

func (inv *Invoicer) CreateStripeInvoice(invoice *Invoice) (string, error) {
	// Create Stripe Invoice
	// Add line items
	// Set payment terms (net-30)
	// Return invoice ID
}
```

### Step 4: Stripe Webhook Handler

```go
func (h *BillingHandler) HandleStripeWebhook(payload []byte) error {
	event := stripe.Event{}
	json.Unmarshal(payload, &event)
	
	switch event.Type {
	case "invoice.paid":
		// Mark invoice as paid in DB
		// Send receipt email
	case "payment_intent.payment_failed":
		// Retry payment or notify customer
	case "customer.subscription.deleted":
		// Deactivate tenant
	}
}
```

---

## Database Schema

```sql
CREATE TABLE billing_events (
  id SERIAL PRIMARY KEY,
  tenant_name VARCHAR(255),
  workload_id VARCHAR(255),
  input_tokens INT,
  output_tokens INT,
  model_tier VARCHAR(50),
  estimated_cost DECIMAL(10, 4),
  actual_cost DECIMAL(10, 4),
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE invoices (
  id VARCHAR(255) PRIMARY KEY,
  tenant_name VARCHAR(255),
  stripe_invoice_id VARCHAR(255),
  period_start DATE,
  period_end DATE,
  subtotal DECIMAL(10, 2),
  tax DECIMAL(10, 2),
  total DECIMAL(10, 2),
  status VARCHAR(50),
  created_at TIMESTAMP,
  paid_at TIMESTAMP
);

CREATE TABLE billing_accounts (
  tenant_name VARCHAR(255) PRIMARY KEY,
  stripe_customer_id VARCHAR(255),
  payment_method VARCHAR(50),
  billing_email VARCHAR(255),
  annual_contract BOOLEAN,
  discount_percent DECIMAL(5, 2),
  is_active BOOLEAN,
  created_at TIMESTAMP
);
```

---

## Tests

**File:** `pkg/billing/billing_test.go`

```go
func TestCostCalculator_BasicCalculation(t *testing.T) {
	// 1K input tokens + 2K output tokens
	// Expected: (1*0.001 + 2*0.002) * 1.10 operator fee
}

func TestCostCalculator_AnnualDiscount(t *testing.T) {
	// Same as above, but with 20% annual discount
	// Expected cost 20% lower
}

func TestInvoicer_MonthlyGeneration(t *testing.T) {
	// Create 10 billing events for March
	// Generate invoice for March 1-31
	// Verify total = sum of all events + operator fee - discount
}

func TestStripeIntegration(t *testing.T) {
	// Create invoice in DB
	// Push to Stripe
	// Verify Stripe invoice ID stored
	// Simulate webhook callback
	// Verify marked as paid
}
```

---

## Success Criteria

✅ Track cost per workload completion  
✅ Calculate monthly invoices automatically  
✅ Integrate with Stripe (create + pay invoices)  
✅ Handle payment webhooks (success + failure)  
✅ Support annual contracts with discounts  
✅ Generate PDF invoices for customers  
✅ Cost alerts (notify when approaching budget)  
✅ All tests passing (unit + integration)  

---

## Integration Points

1. **Controller** (Phase 7-8) — Records BillingEvent on workload completion
2. **SLA Monitor** (Phase 7) — Triggers re-evaluation on cost changes
3. **AutoScaler** (Phase 8) — Avoids scaling when cost budget hit
4. **Tenant Resolution** (Phase 7) — Maps tenant to billing account

---

**Status:** Ready to implement 🚀
