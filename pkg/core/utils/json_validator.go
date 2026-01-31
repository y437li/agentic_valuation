package utils

import (
	"encoding/json"
	"fmt"
	"reflect"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	hjson "github.com/hjson/hjson-go/v4"
)

// ValidateJSON ensures that a json string matches the provided Go struct schema exactly.
// This implements the "Instructor" pattern: using code as the source of truth for LLM output.
func ValidateJSON(jsonData string, schema interface{}) error {
	// 1. Basic Unmarshal check
	err := json.Unmarshal([]byte(jsonData), schema)
	if err != nil {
		return fmt.Errorf("JSON_STRUCTURAL_ERROR: %v", err)
	}

	// 2. Reflection check for missing required fields (not null)
	// In a production environment, we'd use a library like 'go-playground/validator',
	// but for TIED, we implement a strict 'zero-tolerance' check for our core financial fields.
	v := reflect.ValueOf(schema)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := v.Type().Field(i).Name

			// Simple requirement: If it's a financial field (Number or String), it shouldn't be empty/zero
			// unless explicitly allowed by a tag (not implemented in this simplified version).
			if field.IsZero() {
				// We log this specifically so the Agent knows which field was missed by the model.
				return fmt.Errorf("JSON_SCHEMA_VIOLATION: Required field '%s' is missing or zero", fieldName)
			}
		}
	}

	return nil
}

// RepairJSON attempts to fix common JSON errors from LLM outputs.
// Uses github.com/RealAlexandreAI/json-repair for intelligent repair.
// Supported repairs:
// - Missing quotes around keys
// - Single quotes instead of double quotes
// - Unclosed arrays/objects
// - TRUE/FALSE/Null instead of true/false/null
// - Trailing commas
// - Comments in JSON
// - Leading/trailing whitespace and markdown code blocks
func RepairJSON(malformedJSON string) (string, error) {
	repaired, err := jsonrepair.RepairJSON(malformedJSON)
	if err != nil {
		return "", fmt.Errorf("JSON_REPAIR_FAILED: %v", err)
	}
	return repaired, nil
}

// MustRepairJSON is like RepairJSON but returns an empty object on failure.
// Use this in trusted environments or when you need a guaranteed JSON output.
func MustRepairJSON(malformedJSON string) string {
	repaired, err := jsonrepair.RepairJSON(malformedJSON)
	if err != nil {
		return "{}"
	}
	return repaired
}

// ParseHJSON parses Human-friendly JSON (Hjson) and returns standard JSON.
// Hjson supports:
// - Comments (# // /* */)
// - Unquoted keys
// - Unquoted strings
// - Optional commas
// - Multiline strings
//
// This is perfect for parsing human-written configuration or lenient LLM outputs.
func ParseHJSON(hjsonData string) (string, error) {
	var result interface{}
	err := hjson.Unmarshal([]byte(hjsonData), &result)
	if err != nil {
		return "", fmt.Errorf("HJSON_PARSE_ERROR: %v", err)
	}

	// Convert to standard JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON_MARSHAL_ERROR: %v", err)
	}

	return string(jsonBytes), nil
}

// ParseHJSONToStruct parses Hjson directly into a Go struct.
// This is the recommended method when you have a known schema.
func ParseHJSONToStruct(hjsonData string, schema interface{}) error {
	err := hjson.Unmarshal([]byte(hjsonData), schema)
	if err != nil {
		return fmt.Errorf("HJSON_UNMARSHAL_ERROR: %v", err)
	}
	return nil
}

// ValidateAndRepairJSON combines repair and validation into a single workflow.
// This implements the "Draft-Validate-Fix" loop from the json_integrity skill.
// Returns the repaired JSON string and any validation error.
func ValidateAndRepairJSON(rawJSON string, schema interface{}) (string, error) {
	// Step 1: Attempt repair first
	repaired, err := RepairJSON(rawJSON)
	if err != nil {
		// If repair fails, try the original
		repaired = rawJSON
	}

	// Step 2: Validate against schema
	err = ValidateJSON(repaired, schema)
	if err != nil {
		return repaired, err
	}

	return repaired, nil
}

// SmartParse tries multiple parsing strategies to extract valid JSON.
// Order of attempts:
// 1. Standard JSON parse
// 2. JSON repair
// 3. Hjson parse (most lenient)
func SmartParse(input string, schema interface{}) (string, error) {
	// Try 1: Standard JSON
	if err := json.Unmarshal([]byte(input), schema); err == nil {
		return input, nil
	}

	// Try 2: JSON Repair
	repaired, err := RepairJSON(input)
	if err == nil {
		if err := json.Unmarshal([]byte(repaired), schema); err == nil {
			return repaired, nil
		}
	}

	// Try 3: Hjson (most lenient)
	hjsonResult, err := ParseHJSON(input)
	if err == nil {
		if err := json.Unmarshal([]byte(hjsonResult), schema); err == nil {
			return hjsonResult, nil
		}
	}

	return "", fmt.Errorf("SMART_PARSE_FAILED: all parsing strategies failed for input")
}
