package edgar

import (
	"fmt"
	"strings"
)

// ReclassificationEngine manages the active feedback loop between
// Qualitative Insights (Notes) and Quantitative Data (Financial Statements).
type ReclassificationEngine struct {
	// Configuration or thresholds could go here
}

// NewReclassificationEngine creates a new engine instance
func NewReclassificationEngine() *ReclassificationEngine {
	return &ReclassificationEngine{}
}

// ApplyReclassifications executes the "Note modifies Session" logic.
// It scans QualitativeInsights for evidence that suggests line items should be moved.
func (e *ReclassificationEngine) ApplyReclassifications(response *FSAPDataResponse) error {
	if response.Qualitative == nil {
		return nil // No qualitative insights to drive reclassification
	}

	// 1. Reclassify Restructuring Charges
	// Logic: If Note says "We incurred $50M in restructuring", and IS has $50M in Operating Expenses, move it.
	e.reclassifyRestructuring(response)

	// 2. Reclassify Litigation Settlements
	// Logic: Similar logic for legal settlements -> NonRecurring
	e.reclassifyLitigation(response)

	return nil
}

// reclassifyRestructuring handles moving restructuring costs from OpEx to NonRecurring
func (e *ReclassificationEngine) reclassifyRestructuring(response *FSAPDataResponse) {
	// Guard: Need valid pointers
	if response.Qualitative == nil {
		return
	}

	// Scan Strategy Risk Assessment and Top Risks for restructuring context
	hasRestructuringContext := false

	// Check Strategy Risk Assessment
	if strings.Contains(strings.ToLower(response.Qualitative.Strategy.RiskAssessment), "restructuring") {
		hasRestructuringContext = true
	}

	// Check Risk Factors
	for _, risk := range response.Qualitative.Risks.TopRisks {
		if strings.Contains(strings.ToLower(risk.Title), "restructuring") ||
			strings.Contains(strings.ToLower(risk.Summary), "restructuring") {
			hasRestructuringContext = true
			break
		}
	}

	if !hasRestructuringContext {
		return
	}

	// Example heuristic: If the Strategy/Risk Agents identified restructuring context,
	// we actively look for it in the financial statements.

	opSection := response.IncomeStatement.OperatingCostSection
	nrSection := response.IncomeStatement.NonRecurringSection

	if opSection == nil || nrSection == nil {
		return
	}

	// Iterate through AdditionalItems in Operating Cost to find misclassified restructuring
	var remainingItems []AdditionalItem

	for _, item := range opSection.AdditionalItems {
		label := strings.ToLower(item.Label)
		// Check for strong keywords
		if strings.Contains(label, "restructuring") ||
			strings.Contains(label, "severance") ||
			strings.Contains(label, "impairment of asset") {

			// CONFIRMED RECLASSIFICATION
			// 1. Move to NonRecurringSection (Standardized Field if matches, else Additional)
			if strings.Contains(label, "impairment") {
				// Map to ImpairmentCharges if empty
				if nrSection.ImpairmentCharges == nil {
					// Create new FSAPValue from old one
					oldVal := item.Value
					nrSection.ImpairmentCharges = &FSAPValue{
						Value:       oldVal.Value, // *float64
						Label:       item.Label,
						MappingType: "RECLASSIFIED_NOTE_DRIVEN",
						Provenance:  oldVal.Provenance,
					}
					e.logReclassification(response, item.Label, "OperatingCost", "NonRecurring.ImpairmentCharges", "Keyword Match + Note Concurrence")
					continue
				}
			}

			if strings.Contains(label, "restructuring") {
				if nrSection.RestructuringCharges == nil {
					oldVal := item.Value
					nrSection.RestructuringCharges = &FSAPValue{
						Value:       oldVal.Value,
						Label:       item.Label,
						MappingType: "RECLASSIFIED_NOTE_DRIVEN",
						Provenance:  oldVal.Provenance,
					}
					e.logReclassification(response, item.Label, "OperatingCost", "NonRecurring.RestructuringCharges", "Keyword Match + Note Concurrence")
					continue
				}
			}

			// Fallback: Add to NonRecurring AdditionalItems
			nrSection.AdditionalItems = append(nrSection.AdditionalItems, item)
			e.logReclassification(response, item.Label, "OperatingCost", "NonRecurring.Ex", "Keyword Match")

		} else {
			remainingItems = append(remainingItems, item)
		}
	}

	// Update the source section to remove moved items
	opSection.AdditionalItems = remainingItems
}

// reclassifyLitigation handles moving legal settlements
func (e *ReclassificationEngine) reclassifyLitigation(response *FSAPDataResponse) {
	// (Placeholder for similar logic for legal settlements)
}

func (e *ReclassificationEngine) logReclassification(response *FSAPDataResponse, label, from, to, reasoning string) {
	reclass := Reclassification{
		FSAPVariable:         to,
		ReclassificationType: "NOTE_DRIVEN",
		Reasoning:            fmt.Sprintf("Moved '%s' from %s to %s. Reason: %s", label, from, to, reasoning),
		// Value would need to be passed in strictly, simplifying for v1
	}
	response.Reclassifications = append(response.Reclassifications, reclass)
	fmt.Printf("[ReclassificationEngine] %s\n", reclass.Reasoning)
}
