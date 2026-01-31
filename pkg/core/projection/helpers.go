package projection

import (
	"agentic_valuation/pkg/core/edgar"
)

// Helper to safely unpack value
func getValue(v *edgar.FSAPValue) float64 {
	if v != nil && v.Value != nil {
		return *v.Value
	}
	return 0
}
