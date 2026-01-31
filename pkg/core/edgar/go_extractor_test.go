package edgar

import (
	"testing"
)

func TestGoExtractor_ParseNumericValues(t *testing.T) {
	tests := []struct {
		input    string
		expected *float64
	}{
		{"10,000", floatPtr(10000)},
		{"(5,000)", floatPtr(-5000)},
		{"-3,500", floatPtr(-3500)},
		{"$1,234.56", floatPtr(1234.56)},
		{"-", nil},
		{"N/A", nil},
		{"", nil},
		{"100", floatPtr(100)},
	}

	for _, tc := range tests {
		result := parseNumericValueFromString(tc.input)
		if tc.expected == nil {
			if result != nil {
				t.Errorf("Input %q: expected nil, got %f", tc.input, *result)
			}
		} else {
			if result == nil {
				t.Errorf("Input %q: expected %f, got nil", tc.input, *tc.expected)
			} else if *result != *tc.expected {
				t.Errorf("Input %q: expected %f, got %f", tc.input, *tc.expected, *result)
			}
		}
	}

	t.Log("✅ parseNumericValueFromString passed all cases")
}

func floatPtr(f float64) *float64 {
	return &f
}
