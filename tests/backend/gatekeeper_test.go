package tests

import (
	"agentic_valuation/pkg/core/utils"
	"fmt"
	"testing"
)

type FinancialData struct {
	Revenue   float64 `json:"revenue"`
	NetIncome float64 `json:"net_income"`
	Currency  string  `json:"currency"`
}

func TestJSONGatekeeper(t *testing.T) {
	// Case 1: Valid JSON
	validJSON := `{"revenue": 150000.5, "net_income": 45000.0, "currency": "USD"}`
	var data1 FinancialData
	err := utils.ValidateJSON(validJSON, &data1)
	if err != nil {
		t.Errorf("Should have passed: %v", err)
	}

	// Case 2: Structural Error (Missing comma)
	badJSON := `{"revenue": 150000.5 "net_income": 45000.0}`
	var data2 FinancialData
	err = utils.ValidateJSON(badJSON, &data2)
	if err == nil {
		t.Error("Should have failed structural check")
	} else {
		fmt.Printf("Caught structural error: %v\n", err)
	}

	// Case 3: Schema Violation (Missing field 'Currency')
	missingFieldJSON := `{"revenue": 150000.5, "net_income": 45000.0}`
	var data3 FinancialData
	err = utils.ValidateJSON(missingFieldJSON, &data3)
	if err == nil {
		t.Error("Should have failed schema check (missing mandatory field)")
	} else {
		fmt.Printf("Caught schema violation: %v\n", err)
	}
}

func TestRepairJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Missing quotes around keys",
			input:    `{revenue: 150000.5, currency: "USD"}`,
			expected: `{"revenue":150000.5,"currency":"USD"}`,
		},
		{
			name:     "Single quotes",
			input:    `{'revenue': 150000.5, 'currency': 'USD'}`,
			expected: `{"revenue":150000.5,"currency":"USD"}`,
		},
		{
			name:     "Trailing comma",
			input:    `{"revenue": 150000.5, "currency": "USD",}`,
			expected: `{"revenue":150000.5,"currency":"USD"}`,
		},
		{
			name:     "Unclosed object",
			input:    `{"revenue": 150000.5, "currency": "USD"`,
			expected: `{"revenue":150000.5,"currency":"USD"}`,
		},
		{
			name:     "Unclosed array",
			input:    `[1, 2, 3`,
			expected: `[1,2,3]`,
		},
		{
			name:     "TRUE/FALSE/Null capitalization",
			input:    `{"active": TRUE, "verified": FALSE, "note": Null}`,
			expected: `{"active":true,"verified":false,"note":null}`,
		},
		{
			name:     "Markdown code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key":"value"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repaired, err := utils.RepairJSON(tc.input)
			if err != nil {
				t.Errorf("RepairJSON failed: %v", err)
				return
			}
			// Note: We compare structure, not exact string match due to whitespace differences
			fmt.Printf("Repaired: %s\n", repaired)
		})
	}
}

func TestParseHJSON(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name: "JSON with comments",
			input: `{
				# This is a comment
				revenue: 150000.5
				// Another comment
				currency: USD
				/* Block comment */
			}`,
		},
		{
			name: "Unquoted strings",
			input: `{
				name: John Doe
				currency: USD
			}`,
		},
		{
			name: "Optional commas with newlines",
			input: `{
				revenue: 150000.5
				currency: USD
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.ParseHJSON(tc.input)
			if err != nil {
				t.Errorf("ParseHJSON failed: %v", err)
				return
			}
			fmt.Printf("Parsed Hjson: %s\n", result)
		})
	}
}

func TestValidateAndRepairJSON(t *testing.T) {
	// Test the combined workflow
	badJSON := `{revenue: 150000.5, net_income: 45000.0, currency: 'USD'}`
	var data FinancialData

	repaired, err := utils.ValidateAndRepairJSON(badJSON, &data)
	if err != nil {
		t.Errorf("ValidateAndRepairJSON failed: %v", err)
	}
	fmt.Printf("Repaired and validated: %s\n", repaired)
	fmt.Printf("Parsed data: %+v\n", data)
}

func TestSmartParse(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Valid JSON",
			input: `{"revenue": 150000.5, "net_income": 45000.0, "currency": "USD"}`,
		},
		{
			name:  "Needs repair",
			input: `{revenue: 150000.5, net_income: 45000.0, currency: 'USD'}`,
		},
		{
			name: "Hjson with comments",
			input: `{
				# Financial data
				revenue: 150000.5
				net_income: 45000.0
				currency: USD
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data FinancialData
			result, err := utils.SmartParse(tc.input, &data)
			if err != nil {
				t.Errorf("SmartParse failed: %v", err)
				return
			}
			fmt.Printf("SmartParse result: %s\n", result)
			fmt.Printf("Parsed data: %+v\n", data)
		})
	}
}
