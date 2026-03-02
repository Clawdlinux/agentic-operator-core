package autoscaling

import (
	"errors"
	"fmt"
)

var (
	ErrNoDowngradePath = errors.New("no cheaper model available")
	ErrNoUpgradePath   = errors.New("no more expensive model available")
)

// NewModelSelector creates a selector with available tiers and cost config.
func NewModelSelector(startTier ModelTier, availableTiers []ModelTier, costConfig map[ModelTier]float64) *ModelSelector {
	return &ModelSelector{
		CurrentTier:    startTier,
		AvailableTiers: availableTiers,
		CostPerToken:   costConfig,
	}
}

// DefaultCostConfig returns typical pricing (in USD per 1K tokens).
func DefaultCostConfig() map[ModelTier]float64 {
	return map[ModelTier]float64{
		TierCheap:     0.001,
		TierMedium:    0.010,
		TierExpensive: 0.030,
	}
}

// Downgrade moves to next cheaper tier.
func (ms *ModelSelector) Downgrade() (ModelTier, error) {
	switch ms.CurrentTier {
	case TierExpensive:
		if ms.hasTier(TierMedium) {
			ms.CurrentTier = TierMedium
			return TierMedium, nil
		}
		if ms.hasTier(TierCheap) {
			ms.CurrentTier = TierCheap
			return TierCheap, nil
		}
	case TierMedium:
		if ms.hasTier(TierCheap) {
			ms.CurrentTier = TierCheap
			return TierCheap, nil
		}
	}
	return "", ErrNoDowngradePath
}

// Upgrade moves to next more expensive tier.
func (ms *ModelSelector) Upgrade() (ModelTier, error) {
	switch ms.CurrentTier {
	case TierCheap:
		if ms.hasTier(TierMedium) {
			ms.CurrentTier = TierMedium
			return TierMedium, nil
		}
		if ms.hasTier(TierExpensive) {
			ms.CurrentTier = TierExpensive
			return TierExpensive, nil
		}
	case TierMedium:
		if ms.hasTier(TierExpensive) {
			ms.CurrentTier = TierExpensive
			return TierExpensive, nil
		}
	}
	return "", ErrNoUpgradePath
}

// CostSavings estimates monthly savings of downgrading (assuming 1B tokens/month).
func (ms *ModelSelector) CostSavings() float64 {
	current := ms.CostPerToken[ms.CurrentTier]
	if cheaper, err := ms.Downgrade(); err == nil {
		cheaper := ms.CostPerToken[cheaper]
		return (current - cheaper) * 1_000_000 // 1B tokens
	}
	return 0
}

// hasTier checks if tier is available.
func (ms *ModelSelector) hasTier(tier ModelTier) bool {
	for _, t := range ms.AvailableTiers {
		if t == tier {
			return true
		}
	}
	return false
}

// String returns human-readable tier info.
func (ms *ModelSelector) String() string {
	cost := ms.CostPerToken[ms.CurrentTier]
	return fmt.Sprintf("Tier: %s (cost: $%.4f/1K tokens)", ms.CurrentTier, cost)
}
