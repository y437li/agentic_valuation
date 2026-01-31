package ingest

// ExcelCell represents a value at a specific coordinate
type ExcelCell struct {
	Sheet string
	Cell  string
	Value interface{}
}

// IngestResult holds the result of the ingestion and mapping process
type IngestResult struct {
	FileName      string
	MappedFields  int
	TotalFields   int
	Confidence    float64
	VarianceItems []VarianceItem
	IntegrityChecks []AuditCheckpoint
}

type VarianceItem struct {
	Field         string
	ExcelValue    string
	WebValue      string
	Variance      string
	MappingReason string // Explained logic
	OriginalCell  string // e.g. "Forecast!B10"
}

// AuditCheckpoint represents a comparison between a reported total (scraped) and a calculated sum (derived)
type AuditCheckpoint struct {
	CheckpointName  string
	ReportedValue   float64
	CalculatedValue float64
	Variance        float64
	Status          string // 'MATCH', 'IMMATERIAL', 'MATERIAL_MISMATCH'
}

// VerifyIntegrity compares derived sums against scraped totals
func VerifyIntegrity(calculated map[string]float64, reported map[string]float64) []AuditCheckpoint {
	checks := []AuditCheckpoint{}
	threshold := 0.05 // 5% tolerance for immaterial differences

	for name, reportVal := range reported {
		// Mock logic: assume key exists in calculated map with same name
		calcVal, exists := calculated[name]
		if !exists {
			continue
		}

		diff := calcVal - reportVal
		status := "MATCH"
		
		if diff != 0 {
			pctDiff := 0.0
			if reportVal != 0 {
				pctDiff = (diff / reportVal) * 100
			}
			if pctDiff > threshold || pctDiff < -threshold {
				status = "MATERIAL_MISMATCH"
			} else {
				status = "IMMATERIAL"
			}
		}

		checks = append(checks, AuditCheckpoint{
			CheckpointName:  name,
			ReportedValue:   reportVal,
			CalculatedValue: calcVal,
			Variance:        diff,
			Status:          status,
		})
	}
	return checks
}

// MapExcelToFSAP performs semantic mapping from Excel keys to FSAP schema
func MapExcelToFSAP(cells []ExcelCell) IngestResult {
	// ... simulated logic ...
	
	// Mock Integrity Check
	mockCalculated := map[string]float64{
		"Total Assets": 1000.0,
		"Total Equity": 490.0,
	}
	mockReported := map[string]float64{
		"Total Assets": 1000.0,     // Perfect match
		"Total Equity": 500.0,      // Variance of -10
	}

	return IngestResult{
		FileName:     "Imported_Model.xlsx",
		MappedFields: 42,
		TotalFields:  150,
		Confidence:   0.94,
		VarianceItems: []VarianceItem{
			{
				Field:         "Revenue (2025E)",
				ExcelValue:    "172.5B",
				WebValue:      "170.2B",
				Variance:      "+2.3B",
				MappingReason: "Identified as primary top-line figure based on 'Total Sales' label and context in Sheet 'Forecast'.",
				OriginalCell:  "Forecast!B10",
			},
			{
				Field:         "Gross Margin",
				ExcelValue:    "14.2%",
				WebValue:      "14.5%",
				Variance:      "-0.3%",
				MappingReason: "Mapped from 'GP %' calculated cell; matched numerical profile of historical margins.",
				OriginalCell:  "Input!E22",
			},
		},
		IntegrityChecks: VerifyIntegrity(mockCalculated, mockReported),
	}
}
