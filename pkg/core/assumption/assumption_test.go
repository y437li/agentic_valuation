package assumption

import (
	"testing"

	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
)

func TestNewAssumptionSet(t *testing.T) {
	as := NewAssumptionSet("case-123", "scenario-base")

	if as.CaseID != "case-123" {
		t.Errorf("expected case ID 'case-123', got '%s'", as.CaseID)
	}
	if as.ScenarioID != "scenario-base" {
		t.Errorf("expected scenario ID 'scenario-base', got '%s'", as.ScenarioID)
	}
	if as.Nodes == nil {
		t.Error("Nodes map should be initialized")
	}
	if as.Skeleton == nil {
		t.Error("Skeleton should be initialized")
	}
}

func TestAssumptionSet_AddNode(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	node := &Node{
		ID:       "rev-growth",
		Label:    "Revenue Growth",
		Variable: "revenue_growth",
		Value:    5.0,
		Unit:     "%",
	}

	err := as.AddNode(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(as.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(as.Nodes))
	}

	// Verify timestamps set
	if node.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestAssumptionSet_AddNodeDuplicate(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	node := &Node{ID: "rev-growth", Label: "Revenue Growth"}
	_ = as.AddNode(node)
	err := as.AddNode(node)

	if err == nil {
		t.Fatal("expected error for duplicate node, got nil")
	}
}

func TestAssumptionSet_AddNodeEmptyID(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	node := &Node{Label: "No ID"}
	err := as.AddNode(node)

	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestAssumptionSet_GetNode(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")
	node := &Node{ID: "rev-growth", Label: "Revenue Growth", Value: 5.0}
	_ = as.AddNode(node)

	retrieved, err := as.GetNode("rev-growth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Value != 5.0 {
		t.Errorf("expected value 5.0, got %.2f", retrieved.Value)
	}
}

func TestAssumptionSet_GetNodeNotFound(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	_, err := as.GetNode("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node, got nil")
	}
}

func TestAssumptionSet_UpdateNode(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")
	node := &Node{ID: "rev-growth", Label: "Revenue Growth", Value: 5.0}
	_ = as.AddNode(node)

	node.Value = 10.0
	err := as.UpdateNode(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ := as.GetNode("rev-growth")
	if retrieved.Value != 10.0 {
		t.Errorf("expected updated value 10.0, got %.2f", retrieved.Value)
	}
}

func TestAssumptionSet_DeleteNode(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")
	node := &Node{ID: "custom-driver", Label: "Custom Driver"}
	_ = as.AddNode(node)

	err := as.DeleteNode("custom-driver")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(as.Nodes) != 0 {
		t.Error("node should be deleted")
	}
}

func TestAssumptionSet_DeleteSkeletonNode(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	// "revenue" is a skeleton ID, should not be deletable
	err := as.DeleteNode("revenue")
	if err == nil {
		t.Fatal("expected error when deleting skeleton node, got nil")
	}
}

func TestAssumptionSet_GetChildren(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")

	parentID := "rev-growth"
	parent := &Node{ID: parentID, Label: "Revenue Growth"}
	child1 := &Node{ID: "rev-blue", Label: "Ford Blue Growth", ParentID: &parentID}
	child2 := &Node{ID: "rev-model-e", Label: "Model e Growth", ParentID: &parentID}
	orphan := &Node{ID: "ebit-margin", Label: "EBIT Margin"} // No parent

	_ = as.AddNode(parent)
	_ = as.AddNode(child1)
	_ = as.AddNode(child2)
	_ = as.AddNode(orphan)

	children := as.GetChildren(parentID)
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestNodeValue_WithCitation(t *testing.T) {
	nv := NodeValue{
		Value:      100.0,
		SourceType: edgar.SourceInternalDB,
		Confidence: ConfidenceHigh,
		Citations: []edgar.Citation{
			{
				AssetID: "asset-10k",
				ChunkID: "chunk-1",
				Snippet: "Revenue was $100M",
				Link:    "https://sec.gov/...",
			},
		},
	}

	if len(nv.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(nv.Citations))
	}
}

func TestAssumptionSet_ToFromJSON(t *testing.T) {
	as := NewAssumptionSet("case-123", "base")
	node := &Node{
		ID:           "rev-growth",
		Label:        "Revenue Growth",
		Value:        5.0,
		Unit:         "%",
		TrendType:    TrendLinear,
		Distribution: DistNormal,
	}
	_ = as.AddNode(node)

	// Serialize
	data, err := as.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	// Deserialize
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}

	if restored.CaseID != as.CaseID {
		t.Errorf("CaseID mismatch")
	}
	if len(restored.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(restored.Nodes))
	}
	if restored.Skeleton == nil {
		t.Error("Skeleton should be recreated after FromJSON")
	}
}

func TestNodeIntegrationWithProjection(t *testing.T) {
	// Verify that assumption nodes work with projection skeleton
	as := NewAssumptionSet("case-123", "base")

	// Check skeleton is initialized
	if as.Skeleton.Revenue == nil {
		t.Error("skeleton Revenue should exist")
	}

	// Verify skeleton node ID check works
	if !projection.IsSkeletonID("revenue") {
		t.Error("'revenue' should be recognized as skeleton ID")
	}
	if projection.IsSkeletonID("custom-driver") {
		t.Error("'custom-driver' should NOT be recognized as skeleton ID")
	}
}
