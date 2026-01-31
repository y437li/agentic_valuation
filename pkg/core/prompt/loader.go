package prompt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// LoadFromDirectory loads all prompts and schemas from a directory structure
// Expected structure:
//
//	baseDir/
//	  prompts/
//	    category1/
//	      prompt1.json
//	    category2/
//	      prompt2.json
//	  schemas/
//	    schema1.json
func LoadFromDirectory(baseDir string) error {
	registry := Get()

	// Load prompts
	promptDir := filepath.Join(baseDir, "prompts")
	if err := loadPrompts(registry, promptDir); err != nil {
		return fmt.Errorf("failed to load prompts: %w", err)
	}

	// Load schemas
	schemaDir := filepath.Join(baseDir, "schemas")
	if err := loadSchemas(registry, schemaDir); err != nil {
		// Schemas are optional, just log warning
		fmt.Printf("[prompt.Loader] Warning: No schemas loaded from %s: %v\n", schemaDir, err)
	}

	fmt.Printf("[prompt.Loader] Loaded %d prompts from %s\n", registry.Count(), baseDir)
	return nil
}

// loadPrompts recursively loads all .json files from the prompts directory
func loadPrompts(r *Registry, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("prompts directory not found: %s", dir)
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var pt PromptTemplate
		if err := json.Unmarshal(data, &pt); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Auto-generate ID from path if not specified
		if pt.ID == "" {
			pt.ID = generateIDFromPath(path, dir)
		}

		// Auto-detect category from folder name if not specified
		if pt.Category == "" {
			pt.Category = detectCategory(path, dir)
		}

		if err := r.Register(&pt); err != nil {
			return fmt.Errorf("failed to register %s: %w", pt.ID, err)
		}

		return nil
	})
}

// loadSchemas loads all schema JSON files
func loadSchemas(r *Registry, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Schemas are optional
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read schema %s: %w", path, err)
		}

		// Schema files can be stored as-is (the JSON is the schema itself)
		baseName := strings.TrimSuffix(filepath.Base(path), ".json")
		schema := &ResponseSchema{
			ID:         baseName,
			Name:       baseName,
			JSONSchema: string(data),
		}

		return r.RegisterSchema(schema)
	})
}

// generateIDFromPath creates a prompt ID from the file path
// e.g., "prompts/extraction/balance_sheet.json" -> "extraction.balance_sheet"
func generateIDFromPath(path string, baseDir string) string {
	relPath, _ := filepath.Rel(baseDir, path)
	relPath = strings.TrimSuffix(relPath, ".json")
	relPath = strings.ReplaceAll(relPath, string(filepath.Separator), ".")
	return relPath
}

// detectCategory extracts the category from the folder structure
func detectCategory(path string, baseDir string) string {
	relPath, _ := filepath.Rel(baseDir, path)
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) > 1 {
		return parts[0]
	}
	return "default"
}

// RenderUserPrompt executes the user prompt template with the given context
func RenderUserPrompt(pt *PromptTemplate, ctx *PromptExecutionContext) (string, error) {
	if pt.UserPromptTmpl == "" {
		return "", nil
	}

	tmpl, err := template.New(pt.ID).Parse(pt.UserPromptTmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx.Variables); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// MustRenderUserPrompt is like RenderUserPrompt but panics on error
func MustRenderUserPrompt(pt *PromptTemplate, ctx *PromptExecutionContext) string {
	result, err := RenderUserPrompt(pt, ctx)
	if err != nil {
		panic(err)
	}
	return result
}
