package pipeline

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"fmt"
	"os"
	"testing"
)

// --- Mocks ---

type MockZipper struct {
	StitchFunc func(ticker, cik string, snapshots []synthesis.ExtractionSnapshot) (*synthesis.GoldenRecord, error)
}

func (m *MockZipper) Stitch(ticker, cik string, snapshots []synthesis.ExtractionSnapshot) (*synthesis.GoldenRecord, error) {
	if m.StitchFunc != nil {
		return m.StitchFunc(ticker, cik, snapshots)
	}
	return &synthesis.GoldenRecord{Ticker: ticker, CIK: cik}, nil
}

type MockAnalyzer struct {
	AnalyzeFunc func(record *synthesis.GoldenRecord) (*analysis.CompanyAnalysis, error)
}

func (m *MockAnalyzer) Analyze(record *synthesis.GoldenRecord) (*analysis.CompanyAnalysis, error) {
	if m.AnalyzeFunc != nil {
		return m.AnalyzeFunc(record)
	}
	return &analysis.CompanyAnalysis{}, nil
}

type MockRepository struct {
	SaveFunc func(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error
}

func (m *MockRepository) Save(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, record, anal)
	}
	return nil
}

type MockExtractor struct {
	ExtractFunc func(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error)
}

func (m *MockExtractor) Extract(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
	if m.ExtractFunc != nil {
		return m.ExtractFunc(ctx, markdown, provider, meta)
	}
	return &edgar.FSAPDataResponse{Company: meta.CompanyName}, nil
}

type MockContentFetcher struct {
	FetchContentFunc func(filing edgar.FilingMetadata) (string, error)
}

func (m *MockContentFetcher) FetchContent(filing edgar.FilingMetadata) (string, error) {
	if m.FetchContentFunc != nil {
		return m.FetchContentFunc(filing)
	}
	return "mock markdown content", nil
}

type MockAIProvider struct{}

func (m *MockAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return "{}", nil
}

// --- Test ---

func TestOrchestrator_RunForCompany(t *testing.T) {
	reportFile, err := os.Create("test_report.txt")
	if err != nil {
		t.Fatalf("Failed to create report file: %v", err)
	}
	defer reportFile.Close()

	reportLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Println(msg)
		reportFile.WriteString(msg + "\n")
	}

	reportLog("--- Starting Test Execution for Orchestrator ---\n")

	type testCase struct {
		name          string
		filings       []edgar.FilingMetadata
		setupMocks    func(*MockZipper, *MockAnalyzer, *MockRepository, *MockExtractor, *MockContentFetcher)
		expectedError string // substring match
	}

	tests := []testCase{
		{
			name: "Success - Happy Path",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp", FiscalYear: 2024},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				// Defaults are fine for success
			},
			expectedError: "",
		},
		{
			name: "Edge Case - No Filings",
			filings: []edgar.FilingMetadata{}, // Empty input
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
			},
			expectedError: "no snapshots extracted",
		},
		{
			name: "Edge Case - Fetch Error (Skip)",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				f.FetchContentFunc = func(filing edgar.FilingMetadata) (string, error) {
					return "", fmt.Errorf("network error")
				}
			},
			expectedError: "no snapshots extracted", // Should result in no snapshots because the only filing failed fetch
		},
		{
			name: "Edge Case - Extraction Error (Skip)",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				e.ExtractFunc = func(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
					return nil, fmt.Errorf("LLM error")
				}
			},
			expectedError: "no snapshots extracted", // Should result in no snapshots
		},
		{
			name: "Edge Case - Synthesis Failure",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				z.StitchFunc = func(ticker, cik string, snapshots []synthesis.ExtractionSnapshot) (*synthesis.GoldenRecord, error) {
					return nil, fmt.Errorf("stitch failed")
				}
			},
			expectedError: "synthesis failed: stitch failed",
		},
		{
			name: "Edge Case - Analysis Failure",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				a.AnalyzeFunc = func(record *synthesis.GoldenRecord) (*analysis.CompanyAnalysis, error) {
					return nil, fmt.Errorf("math error")
				}
			},
			expectedError: "analysis failed: math error",
		},
		{
			name: "Edge Case - Storage Failure",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"},
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				r.SaveFunc = func(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error {
					return fmt.Errorf("db connection lost")
				}
			},
			expectedError: "storage failed: db connection lost",
		},
		{
			name: "Mixed Success - One Fail One Pass",
			filings: []edgar.FilingMetadata{
				{AccessionNumber: "001", CompanyName: "TestCorp"}, // Fails
				{AccessionNumber: "002", CompanyName: "TestCorp"}, // Succeeds
			},
			setupMocks: func(z *MockZipper, a *MockAnalyzer, r *MockRepository, e *MockExtractor, f *MockContentFetcher) {
				e.ExtractFunc = func(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
					if meta.AccessionNumber == "001" {
						return nil, fmt.Errorf("fail 001")
					}
					return &edgar.FSAPDataResponse{Company: "TestCorp"}, nil
				}
				r.SaveFunc = func(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error {
					reportLog("   [Check] Save called successfully for Mixed Success case")
					return nil
				}
			},
			expectedError: "", // Should succeed overall because one snapshot was collected
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reportLog("\nRunning Case: %s", tc.name)

			// Setup
			zipper := &MockZipper{}
			analyzer := &MockAnalyzer{}
			repo := &MockRepository{}
			extractor := &MockExtractor{}
			fetcher := &MockContentFetcher{}
			provider := &MockAIProvider{}

			tc.setupMocks(zipper, analyzer, repo, extractor, fetcher)

			orch := NewPipelineOrchestratorWithDeps(zipper, analyzer, repo, extractor, fetcher, provider)

			// Execute
			err := orch.RunForCompany(context.Background(), "TEST", "000000", tc.filings)

			// Verify
			if tc.expectedError == "" {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
					reportLog("   [FAIL] Unexpected Bug: %v", err)
				} else {
					reportLog("   [PASS] Pipeline executed successfully.")
				}
			} else {
				if err == nil || err.Error() != tc.expectedError {
					// Check for substring if exact match fails, just in case wrapping format changes slightly
					if err != nil && len(err.Error()) > len(tc.expectedError) && err.Error()[len(err.Error())-len(tc.expectedError):] == tc.expectedError {
						// Suffix match (wrapped error)
						reportLog("   [PASS] Caught expected error: %v", err)
					} else {
						t.Errorf("Expected error '%s', got: %v", tc.expectedError, err)
						reportLog("   [FAIL] Expected error '%s', got: %v", tc.expectedError, err)
					}
				} else {
					reportLog("   [PASS] Caught expected error: %v", err)
				}
			}
		})
	}

	reportLog("\n--- End of Test Report ---")
}
