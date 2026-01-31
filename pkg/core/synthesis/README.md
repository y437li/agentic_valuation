# Synthesis Package

This package implements the **Zipper Algorithm** for multi-year time-series data synthesis.

## Core Concept

The Zipper Engine merges data from multiple SEC filings (10-K, 10-K/A, 10-Q) into a single, authoritative "**Golden Record**" per company.

### Architecture Philosophy: Decoupled Extraction & Synthesis

```
┌─────────────────────────────────────────────────────────────────┐
│                     EXTRACTION LAYER (ETL)                       │
│  - Immutable atomic snapshots (one per SEC filing)               │
│  - Stored in Hybrid Vault (PostgreSQL + .cache/)                 │
│  - Data is NEVER modified after extraction                       │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                     SYNTHESIS LAYER (This Package)               │
│  - Mutable, recomputed on-demand                                 │
│  - Merges snapshots using Zipper Algorithm                       │
│  - Detects Restatements & Accounting Changes                     │
│  - Outputs: GoldenRecord (authoritative time-series)             │
└─────────────────────────────────────────────────────────────────┘
```

## Key Rules

### 1. Amendment Dominance
- **10-K/A always wins** over a base 10-K for the same fiscal year.
- Example: If 2024-03-01 has 10-K and 2024-04-15 has 10-K/A for FY2023 → 10-K/A data is used.

### 2. Recency Bias
- For non-amendment comparisons, **newer filing date wins**.
- Example: 2025 10-K provides restated 2023 data → overwrites 2024 10-K's 2023 data.

### 3. Restatement Detection
- When a value for a past year changes, it's logged as a `RestatementLog`.
- Includes: Old/New values, Delta %, Source Accessions.

## Data Structures

```go
// GoldenRecord: The output, authoritative time-series for a company.
type GoldenRecord struct {
    Ticker       string
    CIK          string
    Timeline     map[int]*YearlySnapshot  // Key: Fiscal Year
    Restatements []RestatementLog
}

// ExtractionSnapshot: Input from a single SEC filing.
type ExtractionSnapshot struct {
    FilingMetadata SourceMetadata
    FiscalYear     int
    Data           *edgar.FSAPDataResponse
}
```

## Usage

```go
import "agentic_valuation/pkg/core/synthesis"

// Create engine
zipper := synthesis.NewZipperEngine()

// Load snapshots (from your extraction layer / cache)
snapshots := []synthesis.ExtractionSnapshot{
    {FilingMetadata: meta1, FiscalYear: 2023, Data: data2023},
    {FilingMetadata: meta2, FiscalYear: 2024, Data: data2024},
}

// Stitch into Golden Record
record, err := zipper.Stitch("AAPL", "0000320193", snapshots)

// Access merged data
rev2024 := record.Timeline[2024].IncomeStatement.GrossProfitSection.Revenues.Value

// Check for restatements
for _, r := range record.Restatements {
    fmt.Printf("Restated %s for %d: %.2f → %.2f (%.1f%%)\n",
        r.Item, r.Year, r.OldValue, r.NewValue, r.DeltaPercent)
}
```

## Test Cases

| Case | Description | Expected Behavior |
|------|-------------|-------------------|
| 1 | Normal Restatement | Newer filing's values overwrite older ones; restatement logged |
| 2 | Fallback to Older | Years not covered by newer filings retain older data |
| 3 | Amendment Priority | 10-K/A beats 10-K regardless of filing order |
| 4 | Radical Accounting Change | Large delta triggers significant restatement alert |
| 5 | Outlier Detection | Revenue = 0 flagged for human review |

Run tests:
```bash
go test -v ./pkg/core/synthesis/...
```

## Future Enhancements

- [ ] **Outlier Guard**: Reject suspicious values (e.g., Revenue = 0) pending agent review.
- [ ] **Judge Agent Integration**: LLM-based adjudication for ambiguous conflicts.
- [ ] **Detective Agent Integration**: MD&A-based explanation for significant restatements.
