package edgar

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	coreEdgar "agentic_valuation/pkg/core/edgar"
)

// HandleDebugExtraction handles /api/debug/extract
// It uses the global agentManager defined in handler.go
func HandleDebugExtraction(w http.ResponseWriter, r *http.Request) {
	if agentManager == nil {
		http.Error(w, "Global agentManager not initialized", 500)
		return
	}

	// Hardcoded test path for Apple 2024
	htmlPath := filepath.Join(".cache", "edgar", "html", "0000320193_000032019324000123.md")

	// Try to find ANY file if specific one missing
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		files, _ := os.ReadDir(filepath.Join(".cache", "edgar", "html"))
		if len(files) > 0 {
			htmlPath = filepath.Join(".cache", "edgar", "html", files[0].Name())
		}
	}

	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), 500)
		return
	}

	provider := agentManager.GetProvider("data_extraction")
	if provider == nil {
		http.Error(w, "No data_extraction provider found", 500)
		return
	}

	adapter := coreEdgar.NewLLMAdapter(provider)
	analyzer := coreEdgar.NewLLMAnalyzer(adapter)
	parser := coreEdgar.NewParser() // No args needed now

	// Stream response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)

	flusher, ok := w.(http.Flusher)
	if !ok {
		fmt.Fprintf(w, "Streaming not supported\n")
	}

	fmt.Fprintf(w, ">>> STARTING DEBUG EXTRACTION <<<\n")
	fmt.Fprintf(w, "Input File: %s\n", htmlPath)
	fmt.Fprintf(w, "Input Size: %d bytes\n", len(htmlBytes))
	fmt.Fprintf(w, "LLM Provider: %T\n", provider)
	fmt.Fprintf(w, "---------------------------------------------------\n")
	if ok {
		flusher.Flush()
	}

	start := time.Now()

	// Call the parser
	markdown, err := parser.ExtractWithLLMAgent(r.Context(), string(htmlBytes), analyzer)

	duration := time.Since(start)

	fmt.Fprintf(w, "---------------------------------------------------\n")
	fmt.Fprintf(w, "Extraction finished in %v\n", duration)

	if err != nil {
		fmt.Fprintf(w, "ERROR: %v\n", err)
	} else {
		fmt.Fprintf(w, "SUCCESS!\n")
		fmt.Fprintf(w, "Markdown Length: %d\n", len(markdown))
		fmt.Fprintf(w, "---------------------------------------------------\n")
		if len(markdown) > 2000 {
			fmt.Fprintf(w, "Preview (First 2000 chars):\n%s\n", markdown[:2000])
		} else {
			fmt.Fprintf(w, "Full Content:\n%s\n", markdown)
		}
	}
	if ok {
		flusher.Flush()
	}
}
