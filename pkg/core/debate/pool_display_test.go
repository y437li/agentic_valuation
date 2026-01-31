package debate

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestMaterialPoolStructure demonstrates the integration of Transcript data into the Material Pool.
func TestMaterialPoolStructure(t *testing.T) {
	// 1. Mock Transcript Data
	transcript := Transcript{
		Ticker:        "AAPL",
		CompanyName:   "Apple Inc.",
		Date:          "2024-05-02",
		Content:       "Operator: Good day... [Prepared Remarks] Tim Cook: We set a new revenue record... [Q&A] Analyst A: Can you talk about China? Tim Cook: We remain very confident...",
		FiscalQuarter: "Q2 2024",
		Source:        "HF/glopardo",
	}

	// 2. Mock Material Pool
	// Note: MaterialPool only holds the heavy data. Metadata like Company/FiscalYear is held in SharedContext.
	pool := &MaterialPool{
		TranscriptHistory: []Transcript{transcript},
	}

	// 3. Serialize to JSON
	data, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// 4. Output
	fmt.Println("=== MATERIAL POOL (Transcript Integration Preview) ===")
	fmt.Println(string(data))
	fmt.Println("======================================================")

	if len(pool.TranscriptHistory) == 0 {
		t.Fail()
	}
}
