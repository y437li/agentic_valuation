// Package assumption implements the backend AssumptionSet for financial modeling.
// Syncs with frontend useAssumptionStore for bidirectional data flow.
// Integrates with projection.StandardSkeleton for calculation.
package assumption

import (
	"encoding/json"
	"fmt"
	"time"

	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
)

// =============================================================================
// NODE VALUE (Time Series Entry with Provenance)
// =============================================================================

// ConfidenceLevel indicates data reliability
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "HIGH"
	ConfidenceMedium ConfidenceLevel = "MEDIUM"
	ConfidenceLow    ConfidenceLevel = "LOW"
)

// NodeValue represents a single year's value with full metadata
type NodeValue struct {
	Value      float64              `json:"value"`
	SourceType edgar.DataSourceType `json:"source_type"` // MANUAL, LOCAL_FILE, INTERNAL_DB, WEB_SEARCH
	Confidence ConfidenceLevel      `json:"confidence"`
	Citations  []edgar.Citation     `json:"citations,omitempty"`
	UpdatedAt  time.Time            `json:"updated_at"`
	UpdatedBy  string               `json:"updated_by"` // "USER", "AI", "SYSTEM"
}

// =============================================================================
// ASSUMPTION NODE (Polymorphic Financial Metric)
// =============================================================================

// TrendType matches frontend useAssumptionStore
type TrendType string

const (
	TrendLinear      TrendType = "Linear"
	TrendSCurve      TrendType = "S-Curve"
	TrendManual      TrendType = "Manual"
	TrendConstant    TrendType = "Constant"
	TrendExponential TrendType = "Exponential"
)

// DistributionType for Monte Carlo simulation
type DistributionType string

const (
	DistNormal     DistributionType = "normal"
	DistTriangular DistributionType = "triangular"
	DistUniform    DistributionType = "uniform"
	DistLognormal  DistributionType = "lognormal"
)

// Node represents a single assumption node (maps to frontend ForecastAssumption)
type Node struct {
	ID       string `json:"id"`       // e.g., "rev-growth", "ebit-margin"
	Label    string `json:"label"`    // e.g., "Revenue Growth", "EBIT Margin"
	Variable string `json:"variable"` // e.g., "revenue_growth", "ebit_margin"

	// Current value and unit
	Value float64 `json:"value"`
	Unit  string  `json:"unit"` // "%", "$M", "units"

	// Projection settings
	TrendType       TrendType `json:"trend_type"`
	ProjectionYears int       `json:"projection_years"`
	YearlyValues    []float64 `json:"yearly_values"` // Projected values for each year

	// Hierarchy
	ParentID    *string  `json:"parent_id,omitempty"`
	ChildrenIDs []string `json:"children_ids,omitempty"`
	IsAtomic    bool     `json:"is_atomic"` // Has trend logic (leaf node)

	// Aggregation (for parent nodes)
	AggregationType string `json:"aggregation_type,omitempty"` // "sum", "average", "weighted"

	// Monte Carlo distribution
	Distribution DistributionType `json:"distribution"`
	Mean         float64          `json:"mean"`
	Std          float64          `json:"std"`
	Min          float64          `json:"min"`
	Max          float64          `json:"max"`

	// Time series with provenance
	HistoricalValues map[int]NodeValue `json:"historical_values,omitempty"` // Year -> NodeValue

	// Strategy reference (links to projection.ProjectionStrategy)
	StrategyName   string             `json:"strategy_name,omitempty"`
	StrategyParams map[string]float64 `json:"strategy_params,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// ASSUMPTION SET (Container for All Assumptions in a Case/Scenario)
// =============================================================================

// AssumptionSet holds all assumption nodes for a case/scenario
// Maps to frontend useAssumptionStore state
type AssumptionSet struct {
	CaseID     string           `json:"case_id"`
	ScenarioID string           `json:"scenario_id"`
	Nodes      map[string]*Node `json:"nodes"` // node_id -> Node

	// Link to projection skeleton
	Skeleton *projection.StandardSkeleton `json:"-"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewAssumptionSet creates a new empty assumption set
func NewAssumptionSet(caseID, scenarioID string) *AssumptionSet {
	now := time.Now()
	return &AssumptionSet{
		CaseID:     caseID,
		ScenarioID: scenarioID,
		Nodes:      make(map[string]*Node),
		Skeleton:   projection.NewStandardSkeleton(),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AddNode adds a node to the assumption set
func (as *AssumptionSet) AddNode(node *Node) error {
	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	if _, exists := as.Nodes[node.ID]; exists {
		return fmt.Errorf("node '%s' already exists", node.ID)
	}

	node.CreatedAt = time.Now()
	node.UpdatedAt = node.CreatedAt
	as.Nodes[node.ID] = node
	as.UpdatedAt = time.Now()
	return nil
}

// GetNode retrieves a node by ID
func (as *AssumptionSet) GetNode(id string) (*Node, error) {
	node, ok := as.Nodes[id]
	if !ok {
		return nil, fmt.Errorf("node '%s' not found", id)
	}
	return node, nil
}

// UpdateNode updates an existing node
func (as *AssumptionSet) UpdateNode(node *Node) error {
	if _, exists := as.Nodes[node.ID]; !exists {
		return fmt.Errorf("node '%s' not found", node.ID)
	}

	node.UpdatedAt = time.Now()
	as.Nodes[node.ID] = node
	as.UpdatedAt = time.Now()
	return nil
}

// DeleteNode removes a node (only if not linked to skeleton)
func (as *AssumptionSet) DeleteNode(id string) error {
	// Check if node is linked to skeleton
	if projection.IsSkeletonID(id) {
		return fmt.Errorf("cannot delete skeleton node '%s'", id)
	}

	if _, exists := as.Nodes[id]; !exists {
		return fmt.Errorf("node '%s' not found", id)
	}

	delete(as.Nodes, id)
	as.UpdatedAt = time.Now()
	return nil
}

// GetChildren returns all child nodes of a parent
func (as *AssumptionSet) GetChildren(parentID string) []*Node {
	var children []*Node
	for _, node := range as.Nodes {
		if node.ParentID != nil && *node.ParentID == parentID {
			children = append(children, node)
		}
	}
	return children
}

// ToJSON serializes the assumption set for frontend sync
func (as *AssumptionSet) ToJSON() ([]byte, error) {
	return json.Marshal(as)
}

// FromJSON deserializes an assumption set from frontend
func FromJSON(data []byte) (*AssumptionSet, error) {
	var as AssumptionSet
	if err := json.Unmarshal(data, &as); err != nil {
		return nil, err
	}
	as.Skeleton = projection.NewStandardSkeleton()
	return &as, nil
}
