package edgar

import (
	"testing"
)

func TestReclassificationEngine_Restructuring(t *testing.T) {
	// Helper for pointer
	f := func(v float64) *float64 { return &v }

	// 1. Setup Mock Response
	// FSAPDataResponse has Value type for IncomeStatement, but Pointers for Sections
	response := &FSAPDataResponse{
		IncomeStatement: IncomeStatement{
			OperatingCostSection: &OperatingCostSection{
				AdditionalItems: []AdditionalItem{
					{
						Label: "Restructuring costs",
						Value: &FSAPValue{Value: f(50.0)},
					},
					{
						Label: "Legal fees",
						Value: &FSAPValue{Value: f(10.0)},
					},
				},
			},
			NonRecurringSection: &NonRecurringSection{
				AdditionalItems: []AdditionalItem{},
			},
		},
		Qualitative: &QualitativeInsights{
			Strategy: StrategyAnalysis{
				RiskAssessment: "Management notes significant restructuring risks this year.",
			},
			Risks: RiskAnalysis{
				TopRisks: []RiskFactor{},
			},
		},
		Reclassifications: []Reclassification{},
	}

	// 2. Run Engine
	engine := NewReclassificationEngine()
	if err := engine.ApplyReclassifications(response); err != nil {
		t.Fatalf("engine failed: %v", err)
	}

	// 3. Verify
	// Expect Restructuring costs (50.0) moved to NonRecurring.RestructuringCharges
	nr := response.IncomeStatement.NonRecurringSection
	if nr.RestructuringCharges == nil || nr.RestructuringCharges.Value == nil || *nr.RestructuringCharges.Value != 50.0 {
		t.Errorf("Expected RestructuringCharges to be 50.0, got %v", nr.RestructuringCharges)
	}

	// Expect it removed from OperatingCostSection.AdditionalItems
	for _, item := range response.IncomeStatement.OperatingCostSection.AdditionalItems {
		if item.Label == "Restructuring costs" {
			t.Errorf("Expected 'Restructuring costs' to be removed from OperatingCostSection")
		}
	}

	// Expect Reclassification Log
	if len(response.Reclassifications) != 1 {
		t.Errorf("Expected 1 reclassification log, got %d", len(response.Reclassifications))
	} else {
		if response.Reclassifications[0].FSAPVariable != "NonRecurring.RestructuringCharges" {
			t.Errorf("Expected log target 'NonRecurring.RestructuringCharges', got %s", response.Reclassifications[0].FSAPVariable)
		}
	}
}
