package projection

import (
	"fmt"
	"strings"
)

// =============================================================================
// AI STRATEGY SELECTOR
// Decides which strategy to use based on available data (Dynamic Discovery)
// =============================================================================

// DriverDiscovery represents what data AI found in the source documents
type DriverDiscovery struct {
	NodeID         string   `json:"node_id"`         // e.g., "revenue"
	AvailableData  []string `json:"available_data"`  // e.g., ["revenue", "volume"]
	DerivedMetrics []string `json:"derived_metrics"` // e.g., ["asp"] (calculated)
	SourceEvidence string   `json:"source_evidence"` // Quote from 10-K
	Confidence     float64  `json:"confidence"`      // 0-1
}

// StrategyDecision is the AI's recommendation for a node
type StrategyDecision struct {
	NodeID              string   `json:"node_id"`
	RecommendedStrategy string   `json:"recommended_strategy"`
	RequiredDrivers     []string `json:"required_drivers"`
	Reasoning           string   `json:"reasoning"`
	Confidence          float64  `json:"confidence"`

	// If drivers need to be created
	NewDrivers []*Node `json:"new_drivers,omitempty"`
}

// StrategySelector determines the optimal strategy based on discovered data
// This is called by the AI agent after scanning the 10-K
type StrategySelector struct {
	// Map of known strategy names to constructors
	strategies map[string]func() ProjectionStrategy
}

// NewStrategySelector creates a selector with all built-in strategies
func NewStrategySelector() *StrategySelector {
	return &StrategySelector{
		strategies: map[string]func() ProjectionStrategy{
			"GrowthRate":  func() ProjectionStrategy { return &GrowthStrategy{} },
			"PriceVolume": func() ProjectionStrategy { return &PriceVolumeStrategy{} },
			"UnitCost":    func() ProjectionStrategy { return &UnitCostStrategy{} },
			"Margin":      func() ProjectionStrategy { return &MarginStrategy{} },
		},
	}
}

// SelectStrategy determines the best strategy for a node based on discovered data
// This encodes the "dynamic discovery" logic
func (s *StrategySelector) SelectStrategy(discovery DriverDiscovery) StrategyDecision {
	available := make(map[string]bool)
	for _, d := range discovery.AvailableData {
		available[strings.ToLower(d)] = true
	}
	for _, d := range discovery.DerivedMetrics {
		available[strings.ToLower(d)] = true
	}

	decision := StrategyDecision{
		NodeID:     discovery.NodeID,
		Confidence: discovery.Confidence,
	}

	// Decision tree for Revenue node
	if discovery.NodeID == "revenue" {
		// Check if we have P×Q data
		hasVolume := available["volume"] || available["units"] || available["deliveries"] || available["shipments"]
		hasPrice := available["price"] || available["asp"] || available["average_selling_price"]

		if hasVolume {
			decision.RecommendedStrategy = "PriceVolume"
			decision.RequiredDrivers = []string{"price", "volume"}
			decision.Reasoning = "Found volume/units data - can build P×Q model"

			// Create driver nodes
			decision.NewDrivers = []*Node{
				{
					ID:           "auto_price",
					Name:         "Average Selling Price",
					Type:         NodeTypeDriver,
					StrategyName: "GrowthRate",
					Values:       make(map[int]float64),
					Unit:         "$",
					UpdatedBy:    "AI",
				},
				{
					ID:           "auto_volume",
					Name:         "Units Sold",
					Type:         NodeTypeDriver,
					StrategyName: "GrowthRate",
					Values:       make(map[int]float64),
					Unit:         "units",
					UpdatedBy:    "AI",
				},
			}

			if !hasPrice {
				decision.Reasoning += " (ASP will be derived: Revenue/Volume)"
			}
			return decision
		}

		// Fallback to simple growth
		decision.RecommendedStrategy = "GrowthRate"
		decision.RequiredDrivers = nil
		decision.Reasoning = "No volume data found - using simple growth model"
		return decision
	}

	// Decision tree for COGS node
	if discovery.NodeID == "cogs" {
		// Check if Revenue has volume (can subscribe)
		if available["volume"] || available["units"] {
			decision.RecommendedStrategy = "UnitCost"
			decision.RequiredDrivers = []string{"volume", "unit_cost"}
			decision.Reasoning = "Volume available from Revenue - can build Unit Cost model"

			decision.NewDrivers = []*Node{
				{
					ID:           "auto_unit_cost",
					Name:         "Unit Cost",
					Type:         NodeTypeDriver,
					StrategyName: "GrowthRate",
					Values:       make(map[int]float64),
					Unit:         "$",
					UpdatedBy:    "AI",
				},
			}
			return decision
		}

		// Fallback to margin-based
		decision.RecommendedStrategy = "Margin"
		decision.RequiredDrivers = []string{"revenue"}
		decision.Reasoning = "Using margin % of revenue"
		return decision
	}

	// Default fallback for other nodes
	decision.RecommendedStrategy = "GrowthRate"
	decision.RequiredDrivers = nil
	decision.Reasoning = "Default growth strategy"
	return decision
}

// CreateStrategy instantiates a strategy by name
func (s *StrategySelector) CreateStrategy(name string) (ProjectionStrategy, error) {
	constructor, ok := s.strategies[name]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
	return constructor(), nil
}

// ApplyDecision applies a strategy decision to a skeleton
func (s *StrategySelector) ApplyDecision(skeleton *StandardSkeleton, decision StrategyDecision) error {
	nodes := skeleton.GetAllNodes()
	node, ok := nodes[decision.NodeID]
	if !ok {
		return fmt.Errorf("node '%s' not found in skeleton", decision.NodeID)
	}

	// Create and assign strategy
	strategy, err := s.CreateStrategy(decision.RecommendedStrategy)
	if err != nil {
		return err
	}
	node.Strategy = strategy
	node.StrategyName = decision.RecommendedStrategy
	node.UpdatedBy = "AI"

	// Attach new drivers
	for _, driver := range decision.NewDrivers {
		if err := node.AttachDriver(driver); err != nil {
			return fmt.Errorf("failed to attach driver '%s': %w", driver.ID, err)
		}
	}

	return nil
}
