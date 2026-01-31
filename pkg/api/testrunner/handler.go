// Package testrunner provides API handlers for executing Go tests
package testrunner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TestRequest represents a request to run tests
type TestRequest struct {
	TestID   string `json:"test_id"`   // e.g., "synthesis/e2e_test.go"
	TestName string `json:"test_name"` // e.g., "TestE2EApple2020Plus"
	Package  string `json:"package"`   // e.g., "synthesis"
	Ticker   string `json:"ticker"`    // e.g., "AAPL" - passed via env var
	CIK      string `json:"cik"`       // e.g., "0000320193" - passed via env var
}

// TestResult represents the result of a test execution
type TestResult struct {
	TestID   string  `json:"test_id"`
	Status   string  `json:"status"` // "running", "pass", "fail"
	Duration float64 `json:"duration,omitempty"`
	Output   string  `json:"output,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// TestRunner manages test execution
type TestRunner struct {
	mu       sync.Mutex
	rootPath string // Path to project root
}

// NewTestRunner creates a new test runner
func NewTestRunner(rootPath string) *TestRunner {
	return &TestRunner{
		rootPath: rootPath,
	}
}

// Global instance
var defaultRunner *TestRunner
var once sync.Once

// GetRunner returns the singleton test runner
func GetRunner() *TestRunner {
	once.Do(func() {
		defaultRunner = NewTestRunner(".")
	})
	return defaultRunner
}

// SetRootPath sets the project root path
func (r *TestRunner) SetRootPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rootPath = path
}

// RunTest executes a single test and returns the result
func (r *TestRunner) RunTest(ctx context.Context, req TestRequest) TestResult {
	start := time.Now()

	// Map test_id to package path
	packagePath := r.getPackagePath(req.TestID)

	// Build go test command
	// Format: go test -v -run TestName ./pkg/core/...
	args := []string{"test", "-v", "-timeout", "60s"}

	// Add test name filter if provided
	if req.TestName != "" {
		args = append(args, "-run", req.TestName)
	}

	args = append(args, packagePath)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.rootPath

	// Set environment variables for company selection
	cmd.Env = os.Environ()
	if req.Ticker != "" {
		cmd.Env = append(cmd.Env, "TEST_TICKER="+req.Ticker)
	}
	if req.CIK != "" {
		cmd.Env = append(cmd.Env, "TEST_CIK="+req.CIK)
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	result := TestResult{
		TestID:   req.TestID,
		Duration: duration,
		Output:   string(output),
	}

	if err != nil {
		// Check if it's a test failure vs execution error
		if strings.Contains(string(output), "FAIL") {
			result.Status = "fail"
		} else {
			result.Status = "fail"
			result.Error = err.Error()
		}
	} else {
		result.Status = "pass"
	}

	return result
}

// getPackagePath converts test_id to Go package path
func (r *TestRunner) getPackagePath(testID string) string {
	// testID format: "synthesis/e2e_test.go" or "validate/linkage_test.go"
	parts := strings.Split(testID, "/")
	if len(parts) < 2 {
		return "./pkg/core/..."
	}

	pkg := parts[0]

	// Map package names to paths
	pathMap := map[string]string{
		"synthesis":  "./pkg/core/synthesis",
		"validate":   "./pkg/core/validate",
		"edgar":      "./pkg/core/edgar",
		"calc":       "./pkg/core/calc",
		"projection": "./pkg/core/projection",
	}

	if path, ok := pathMap[pkg]; ok {
		return path
	}
	return "./pkg/core/" + pkg
}

// HandleRunTest handles POST /api/test/run
func HandleRunTest(w http.ResponseWriter, r *http.Request) {
	// Enable CORS first!
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle Preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validate Method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result := GetRunner().RunTest(ctx, req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRunTestStream handles GET /api/test/run-stream for SSE streaming
// This runs a test and streams output in real-time
func HandleRunTestStream(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	testID := r.URL.Query().Get("test_id")
	testName := r.URL.Query().Get("test_name")

	if testID == "" {
		fmt.Fprintf(w, "data: {\"error\": \"test_id required\"}\n\n")
		flusher.Flush()
		return
	}

	runner := GetRunner()
	packagePath := runner.getPackagePath(testID)

	start := time.Now()

	// Send start event
	startEvent := map[string]interface{}{
		"type":    "start",
		"test_id": testID,
	}
	jsonData, _ := json.Marshal(startEvent)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()

	// Build command
	args := []string{"test", "-v", "-timeout", "60s"}
	if testName != "" {
		args = append(args, "-run", testName)
	}
	args = append(args, packagePath)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = runner.rootPath

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errEvent := map[string]interface{}{
			"type":    "error",
			"test_id": testID,
			"error":   err.Error(),
		}
		jsonData, _ := json.Marshal(errEvent)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
		return
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		errEvent := map[string]interface{}{
			"type":    "error",
			"test_id": testID,
			"error":   err.Error(),
		}
		jsonData, _ := json.Marshal(errEvent)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
		return
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputEvent := map[string]interface{}{
				"type":    "output",
				"test_id": testID,
				"line":    line,
			}
			jsonData, _ := json.Marshal(outputEvent)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputEvent := map[string]interface{}{
				"type":    "output",
				"test_id": testID,
				"line":    line,
			}
			jsonData, _ := json.Marshal(outputEvent)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}()

	// Wait for completion
	err = cmd.Wait()
	duration := time.Since(start).Seconds()

	status := "pass"
	if err != nil {
		status = "fail"
	}

	// Send completion event
	completeEvent := map[string]interface{}{
		"type":     "complete",
		"test_id":  testID,
		"status":   status,
		"duration": duration,
	}
	jsonData, _ = json.Marshal(completeEvent)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}
