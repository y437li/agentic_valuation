package projection_test

import (
	"agentic_valuation/pkg/core/projection"
	"testing"
)

func TestGrowthStrategy(t *testing.T) {
	s := &projection.GrowthStrategy{GrowthRate: 0.05} // 5% growth

	ctx := projection.Context{
		Year:          2025,
		LastYearValue: 100.0,
	}

	result, err := s.Calculate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 105.0
	if result != expected {
		t.Errorf("expected %.2f, got %.2f", expected, result)
	}
}

func TestPriceVolumeStrategy(t *testing.T) {
	s := &projection.PriceVolumeStrategy{}

	ctx := projection.Context{
		Year: 2025,
		Drivers: map[string]float64{
			"price":  50.0,
			"volume": 1000.0,
		},
	}

	result, err := s.Calculate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 50000.0 // 50 * 1000
	if result != expected {
		t.Errorf("expected %.2f, got %.2f", expected, result)
	}
}

func TestPriceVolumeStrategy_MissingDriver(t *testing.T) {
	s := &projection.PriceVolumeStrategy{}

	ctx := projection.Context{
		Year: 2025,
		Drivers: map[string]float64{
			"price": 50.0,
			// missing "volume"
		},
	}

	_, err := s.Calculate(ctx)
	if err == nil {
		t.Fatal("expected error for missing driver, got nil")
	}
}

func TestUnitCostStrategy(t *testing.T) {
	s := &projection.UnitCostStrategy{}

	ctx := projection.Context{
		Year: 2025,
		Drivers: map[string]float64{
			"volume":    1000.0,
			"unit_cost": 30.0,
		},
	}

	result, err := s.Calculate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 30000.0 // 1000 * 30
	if result != expected {
		t.Errorf("expected %.2f, got %.2f", expected, result)
	}
}

func TestMarginStrategy(t *testing.T) {
	s := &projection.MarginStrategy{
		MarginPercent: 0.40, // 40% margin
		BaseNodeID:    "revenue",
	}

	ctx := projection.Context{
		Year: 2025,
		Drivers: map[string]float64{
			"revenue": 100000.0,
		},
	}

	result, err := s.Calculate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 40000.0 // 100000 * 0.40
	if result != expected {
		t.Errorf("expected %.2f, got %.2f", expected, result)
	}
}

func TestNewStandardSkeleton(t *testing.T) {
	skeleton := projection.NewStandardSkeleton()

	// Verify all nodes exist
	if skeleton.Revenue == nil {
		t.Error("Revenue node is nil")
	}
	if skeleton.COGS == nil {
		t.Error("COGS node is nil")
	}
	if skeleton.NetIncome == nil {
		t.Error("NetIncome node is nil")
	}

	// Verify node type
	if skeleton.Revenue.Type != projection.NodeTypeSkeleton {
		t.Errorf("expected SKELETON type, got %s", skeleton.Revenue.Type)
	}

	// Verify default strategy
	if skeleton.Revenue.StrategyName != "GrowthRate" {
		t.Errorf("expected GrowthRate strategy, got %s", skeleton.Revenue.StrategyName)
	}
}

func TestNodeAttachDriver(t *testing.T) {
	skeleton := projection.NewStandardSkeleton()

	driver := &projection.Node{
		ID:   "test_driver",
		Name: "Test Driver",
		Type: projection.NodeTypeDriver,
	}

	err := skeleton.Revenue.AttachDriver(driver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skeleton.Revenue.DriverIDs) != 1 {
		t.Errorf("expected 1 driver, got %d", len(skeleton.Revenue.DriverIDs))
	}

	if skeleton.Revenue.DriverIDs[0] != "test_driver" {
		t.Errorf("expected 'test_driver', got '%s'", skeleton.Revenue.DriverIDs[0])
	}

	// Verify parent link
	if driver.ParentID == nil || *driver.ParentID != "revenue" {
		t.Error("driver parent ID not set correctly")
	}
}

func TestNodeAttachDriver_DuplicatePrevented(t *testing.T) {
	skeleton := projection.NewStandardSkeleton()

	driver := &projection.Node{
		ID:   "test_driver",
		Name: "Test Driver",
		Type: projection.NodeTypeDriver,
	}

	_ = skeleton.Revenue.AttachDriver(driver)
	err := skeleton.Revenue.AttachDriver(driver)

	if err == nil {
		t.Fatal("expected error for duplicate driver, got nil")
	}
}

func TestIsSkeletonID(t *testing.T) {
	if !projection.IsSkeletonID("revenue") {
		t.Error("'revenue' should be a skeleton ID")
	}
	if !projection.IsSkeletonID("cogs") {
		t.Error("'cogs' should be a skeleton ID")
	}
	if projection.IsSkeletonID("auto_price") {
		t.Error("'auto_price' should NOT be a skeleton ID")
	}
}

func TestStrategySelector_RevenueWithVolume(t *testing.T) {
	selector := projection.NewStrategySelector()

	discovery := projection.DriverDiscovery{
		NodeID:        "revenue",
		AvailableData: []string{"revenue", "volume"},
		Confidence:    0.9,
	}

	decision := selector.SelectStrategy(discovery)

	if decision.RecommendedStrategy != "PriceVolume" {
		t.Errorf("expected PriceVolume, got %s", decision.RecommendedStrategy)
	}

	if len(decision.NewDrivers) != 2 {
		t.Errorf("expected 2 new drivers, got %d", len(decision.NewDrivers))
	}
}

func TestStrategySelector_RevenueWithoutVolume(t *testing.T) {
	selector := projection.NewStrategySelector()

	discovery := projection.DriverDiscovery{
		NodeID:        "revenue",
		AvailableData: []string{"revenue"},
		Confidence:    0.9,
	}

	decision := selector.SelectStrategy(discovery)

	if decision.RecommendedStrategy != "GrowthRate" {
		t.Errorf("expected GrowthRate, got %s", decision.RecommendedStrategy)
	}

	if len(decision.NewDrivers) != 0 {
		t.Errorf("expected 0 new drivers, got %d", len(decision.NewDrivers))
	}
}

func TestStrategySelector_ApplyDecision(t *testing.T) {
	selector := projection.NewStrategySelector()
	skeleton := projection.NewStandardSkeleton()

	discovery := projection.DriverDiscovery{
		NodeID:        "revenue",
		AvailableData: []string{"revenue", "deliveries"},
		Confidence:    0.95,
	}

	decision := selector.SelectStrategy(discovery)
	err := selector.ApplyDecision(skeleton, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify strategy was updated
	if skeleton.Revenue.StrategyName != "PriceVolume" {
		t.Errorf("expected PriceVolume, got %s", skeleton.Revenue.StrategyName)
	}

	// Verify drivers were attached
	if len(skeleton.Revenue.DriverIDs) != 2 {
		t.Errorf("expected 2 drivers, got %d", len(skeleton.Revenue.DriverIDs))
	}

	// Verify UpdatedBy
	if skeleton.Revenue.UpdatedBy != "AI" {
		t.Errorf("expected UpdatedBy='AI', got '%s'", skeleton.Revenue.UpdatedBy)
	}
}
