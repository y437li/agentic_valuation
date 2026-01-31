// Package projection implements the Polymorphic Node system for AI-native financial modeling.
// Core Philosophy: "Fixed Skeleton, Dynamic Drivers"
// - Skeleton (Parent Nodes): Fixed by Go code, ensures accounting identity
// - Drivers (Child Nodes): AI-discovered, dynamically attached to parent nodes
package projection

import (
	"fmt"
	"time"
)

// =============================================================================
// PROJECTION STRATEGY INTERFACE
// =============================================================================

// Context provides data needed for strategy calculations
type Context struct {
	Year           int                // Target projection year
	LastYearValue  float64            // Previous year's value for this node
	HistoricalData map[int]float64    // All historical values
	Drivers        map[string]float64 // Values from child driver nodes
}

// ProjectionStrategy defines a pluggable forecasting algorithm
// AI selects the appropriate strategy based on available data
type ProjectionStrategy interface {
	// Name returns the strategy identifier
	Name() string

	// Calculate computes the projected value for a given context
	Calculate(ctx Context) (float64, error)

	// RequiredDrivers returns the driver node IDs needed by this strategy
	// Empty slice = no drivers needed (e.g., simple growth)
	RequiredDrivers() []string

	// Validate checks if necessary inputs are available
	Validate(ctx Context) error
}

// =============================================================================
// BUILT-IN STRATEGIES
// =============================================================================

// GrowthStrategy implements simple percentage growth (default fallback)
// Formula: Value(t) = Value(t-1) * (1 + GrowthRate)
type GrowthStrategy struct {
	GrowthRate float64 `json:"growth_rate"` // e.g., 0.05 for 5%
}

func (s *GrowthStrategy) Name() string { return "GrowthRate" }

func (s *GrowthStrategy) RequiredDrivers() []string { return nil }

func (s *GrowthStrategy) Validate(ctx Context) error {
	if ctx.LastYearValue == 0 {
		return fmt.Errorf("GrowthStrategy requires LastYearValue")
	}
	return nil
}

func (s *GrowthStrategy) Calculate(ctx Context) (float64, error) {
	if err := s.Validate(ctx); err != nil {
		return 0, err
	}
	return ctx.LastYearValue * (1 + s.GrowthRate), nil
}

// PriceVolumeStrategy implements P × Q decomposition
// Formula: Revenue = Price × Volume
// AI auto-derives: ASP = Revenue / Volume (when only Revenue & Volume known)
type PriceVolumeStrategy struct {
	// These are typically overridden by driver nodes
	PriceGrowth  float64 `json:"price_growth"`  // e.g., 0.02 for 2%
	VolumeGrowth float64 `json:"volume_growth"` // e.g., 0.03 for 3%
}

func (s *PriceVolumeStrategy) Name() string { return "PriceVolume" }

func (s *PriceVolumeStrategy) RequiredDrivers() []string {
	return []string{"price", "volume"}
}

func (s *PriceVolumeStrategy) Validate(ctx Context) error {
	if _, ok := ctx.Drivers["price"]; !ok {
		return fmt.Errorf("PriceVolumeStrategy requires 'price' driver")
	}
	if _, ok := ctx.Drivers["volume"]; !ok {
		return fmt.Errorf("PriceVolumeStrategy requires 'volume' driver")
	}
	return nil
}

func (s *PriceVolumeStrategy) Calculate(ctx Context) (float64, error) {
	if err := s.Validate(ctx); err != nil {
		return 0, err
	}
	return ctx.Drivers["price"] * ctx.Drivers["volume"], nil
}

// UnitCostStrategy implements Volume × Unit Cost
// Subscribes to Volume from Revenue's PriceVolumeStrategy
// Formula: Cost = Volume × UnitCost
type UnitCostStrategy struct {
	UnitCostGrowth float64 `json:"unit_cost_growth"` // e.g., 0.01 for 1%
}

func (s *UnitCostStrategy) Name() string { return "UnitCost" }

func (s *UnitCostStrategy) RequiredDrivers() []string {
	return []string{"volume", "unit_cost"}
}

func (s *UnitCostStrategy) Validate(ctx Context) error {
	if _, ok := ctx.Drivers["volume"]; !ok {
		return fmt.Errorf("UnitCostStrategy requires 'volume' driver (can subscribe from Revenue)")
	}
	if _, ok := ctx.Drivers["unit_cost"]; !ok {
		return fmt.Errorf("UnitCostStrategy requires 'unit_cost' driver")
	}
	return nil
}

func (s *UnitCostStrategy) Calculate(ctx Context) (float64, error) {
	if err := s.Validate(ctx); err != nil {
		return 0, err
	}
	return ctx.Drivers["volume"] * ctx.Drivers["unit_cost"], nil
}

// MarginStrategy implements Margin % of Base
// Formula: Value = BaseValue × MarginPercent
// Example: COGS = Revenue × (1 - GrossMargin%)
type MarginStrategy struct {
	MarginPercent float64 `json:"margin_percent"` // e.g., 0.40 for 40%
	BaseNodeID    string  `json:"base_node_id"`   // e.g., "revenue"
}

func (s *MarginStrategy) Name() string { return "Margin" }

func (s *MarginStrategy) RequiredDrivers() []string {
	return []string{s.BaseNodeID}
}

func (s *MarginStrategy) Validate(ctx Context) error {
	if _, ok := ctx.Drivers[s.BaseNodeID]; !ok {
		return fmt.Errorf("MarginStrategy requires '%s' base value", s.BaseNodeID)
	}
	return nil
}

func (s *MarginStrategy) Calculate(ctx Context) (float64, error) {
	if err := s.Validate(ctx); err != nil {
		return 0, err
	}
	return ctx.Drivers[s.BaseNodeID] * s.MarginPercent, nil
}

// SumStrategy implements summation of all child drivers
// Formula: ParentValue = Sum(ChildValues)
// Use Case: Total Revenue = Segment A + Segment B + ...
type SumStrategy struct {
	// No specific params needed, it sums all attached drivers
}

func (s *SumStrategy) Name() string { return "Sum" }

// RequiredDrivers returns nil because it uses *all* attached drivers dynamically
// The Context.Drivers map is populated with all children by the engine
func (s *SumStrategy) RequiredDrivers() []string { return nil }

func (s *SumStrategy) Validate(ctx Context) error {
	// Optional: Could require at least one driver?
	// For now, allow empty sum = 0
	return nil
}

func (s *SumStrategy) Calculate(ctx Context) (float64, error) {
	total := 0.0
	for _, val := range ctx.Drivers {
		total += val
	}
	return total, nil
}

// =============================================================================
// POLYMORPHIC NODE
// =============================================================================

// NodeType distinguishes between fixed skeleton nodes and dynamic drivers
type NodeType string

const (
	NodeTypeSkeleton NodeType = "SKELETON" // Fixed by Go, cannot be deleted by AI
	NodeTypeDriver   NodeType = "DRIVER"   // AI-discovered, can be attached/detached
)

// Node represents a polymorphic financial metric
// Fixed nodes (Skeleton): Revenue, COGS, GrossProfit, etc.
// Dynamic nodes (Drivers): Price, Volume, ARPU, etc.
type Node struct {
	ID   string   `json:"id"`   // e.g., "revenue", "cogs", "auto_price"
	Name string   `json:"name"` // e.g., "Total Revenue", "Cost of Goods Sold"
	Type NodeType `json:"type"` // SKELETON or DRIVER

	// Strategy determines how this node is calculated
	// Default = GrowthStrategy for skeleton nodes
	Strategy       ProjectionStrategy `json:"-"` // Not serialized directly
	StrategyName   string             `json:"strategy_name"`
	StrategyParams map[string]float64 `json:"strategy_params,omitempty"`

	// Hierarchy
	ParentID     *string  `json:"parent_id,omitempty"`     // For drivers attached to skeleton
	DriverIDs    []string `json:"driver_ids,omitempty"`    // Child drivers (AI can attach)
	SubscribesTo []string `json:"subscribes_to,omitempty"` // Cross-reference (e.g., Cost subscribes to Revenue.Volume)

	// Time series values
	Values map[int]float64 `json:"values"` // Year -> Value

	// Metadata
	Unit      string    `json:"unit,omitempty"` // "%", "$M", "units"
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"` // "USER", "AI", "SYSTEM"
}

// AttachDriver adds a dynamic driver node to this skeleton node
// Returns error if this node is not a skeleton or driver already exists
func (n *Node) AttachDriver(driver *Node) error {
	if n.Type != NodeTypeSkeleton {
		return fmt.Errorf("can only attach drivers to skeleton nodes")
	}
	if driver.Type != NodeTypeDriver {
		return fmt.Errorf("can only attach driver-type nodes")
	}

	// Check for duplicate
	for _, id := range n.DriverIDs {
		if id == driver.ID {
			return fmt.Errorf("driver '%s' already attached", driver.ID)
		}
	}

	n.DriverIDs = append(n.DriverIDs, driver.ID)
	driver.ParentID = &n.ID
	return nil
}

// DetachDriver removes a dynamic driver from this node
func (n *Node) DetachDriver(driverID string) error {
	for i, id := range n.DriverIDs {
		if id == driverID {
			n.DriverIDs = append(n.DriverIDs[:i], n.DriverIDs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("driver '%s' not found", driverID)
}
