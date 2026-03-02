package autoscaling

import "testing"

func TestModelSelectorDowngrade(t *testing.T) {
	costs := DefaultCostConfig()
	ms := NewModelSelector(TierExpensive, []ModelTier{TierExpensive, TierMedium, TierCheap}, costs)
	tier, err := ms.Downgrade()
	if err != nil {
		t.Fatalf("Downgrade failed: %v", err)
	}
	if tier != TierMedium {
		t.Errorf("expected TierMedium, got %v", tier)
	}
}

func TestModelSelectorUpgrade(t *testing.T) {
	costs := DefaultCostConfig()
	ms := NewModelSelector(TierCheap, []ModelTier{TierCheap, TierMedium, TierExpensive}, costs)
	tier, err := ms.Upgrade()
	if err != nil {
		t.Fatalf("Upgrade failed: %v", err)
	}
	if tier != TierMedium {
		t.Errorf("expected TierMedium, got %v", tier)
	}
}

func TestModelSelectorDowngradeLimit(t *testing.T) {
	costs := DefaultCostConfig()
	ms := NewModelSelector(TierCheap, []ModelTier{TierCheap}, costs)
	_, err := ms.Downgrade()
	if err != ErrNoDowngradePath {
		t.Errorf("expected ErrNoDowngradePath, got %v", err)
	}
}

func TestModelSelectorCostSavings(t *testing.T) {
	costs := DefaultCostConfig()
	ms := NewModelSelector(TierExpensive, []ModelTier{TierExpensive, TierMedium, TierCheap}, costs)
	savings := ms.CostSavings()
	if savings <= 0 {
		t.Errorf("expected positive savings, got %.0f", savings)
	}
}
