package main

import (
	"agentic_valuation/pkg/api/assistant"
	"agentic_valuation/pkg/api/config"
	"agentic_valuation/pkg/api/debate"
	"agentic_valuation/pkg/api/edgar"
	"agentic_valuation/pkg/api/testrunner"
	"agentic_valuation/pkg/api/valuation"
	"agentic_valuation/pkg/core/agent"
	coreDebate "agentic_valuation/pkg/core/debate"
	"agentic_valuation/pkg/core/prompt"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

var agentMgr *agent.Manager

func main() {
	// Load environment variables
	godotenv.Load()

	// Initialize Prompt Library
	// Determine resources path (relative to executable or working directory)
	resourcesPath := "resources"
	if _, err := os.Stat(resourcesPath); os.IsNotExist(err) {
		// Try from executable directory
		exePath, _ := os.Executable()
		resourcesPath = filepath.Join(filepath.Dir(exePath), "resources")
	}
	if err := prompt.LoadFromDirectory(resourcesPath); err != nil {
		fmt.Printf("[WARNING] Failed to load prompt library: %v\n", err)
		fmt.Println("  Falling back to hardcoded prompts")
	} else {
		fmt.Printf("[PROMPT] Loaded %d prompts from %s\n", prompt.Get().Count(), resourcesPath)
	}

	// Initialize manager from config
	configData, _ := ioutil.ReadFile("config/models.yaml")
	var agentCfg agent.Config
	yaml.Unmarshal(configData, &agentCfg)
	agentMgr = agent.NewManager(agentCfg)

	// Initialize EDGAR handler with agent manager for LLM analysis
	edgar.InitHandler(agentMgr)

	// Config endpoints
	configHandler := config.NewHandler(agentMgr)
	http.HandleFunc("/api/config", configHandler.HandleConfig)
	http.HandleFunc("/api/config/switch", configHandler.HandleSwitch)

	// AI Assistant endpoints
	assistantHandler := assistant.NewHandler(agentMgr)
	http.HandleFunc("/api/assistant/navigate", assistantHandler.HandleNavigationIntent)

	// EDGAR mapping endpoints
	// http.HandleFunc("/api/edgar/map", edgar.HandleEdgarMapping) // Legacy removed
	http.HandleFunc("/api/edgar/fsap-map", edgar.HandleEdgarFSAPMapping)
	http.HandleFunc("/api/edgar/fsap-map-stream", edgar.HandleEdgarFSAPMapStream(agentMgr))
	http.HandleFunc("/api/edgar/clear-cache", edgar.HandleClearCache)
	http.HandleFunc("/api/debug/extract", edgar.HandleDebugExtraction) // DEBUG

	// Valuation endpoints
	valuation.InitHandler(agentMgr)
	http.HandleFunc("/api/valuation/report", valuation.HandleValuationReport)

	// Initialize Debate Manager with Agent Manager
	fmt.Println("Initializing Debate Manager...")
	coreDebate.GetManager().SetAgentManager(agentMgr)

	// Multi-Agent Debate endpoints
	fmt.Println("Registering Debate Endpoints...")
	http.HandleFunc("/api/debate/active", debate.HandleActiveDebates)
	http.HandleFunc("/api/debate/start", debate.HandleStartDebate)
	http.HandleFunc("/api/debate/stream", debate.HandleStreamDebate)
	http.HandleFunc("/api/debate/question", debate.HandleSubmitQuestion) // New: Human Q&A
	http.HandleFunc("/api/debate/resume", debate.HandleResumeDebate)     // New: Resume debate
	fmt.Println("Debate Endpoints Registered.")

	// Test Runner endpoints (for E2E Workbench)
	http.HandleFunc("/api/test/run", testrunner.HandleRunTest)
	http.HandleFunc("/api/test/run-stream", testrunner.HandleRunTestStream)
	fmt.Println("Test Runner Endpoints Registered.")

	fmt.Println("API server starting on :8080...")
	fmt.Println("  - GET  /api/config")
	fmt.Println("  - POST /api/config/switch")
	fmt.Println("  - POST /api/assistant/navigate  (NEW: AI Navigation)")
	fmt.Println("  - POST /api/edgar/map")
	fmt.Println("  - POST /api/edgar/fsap-map  (NEW: FSAP format with source_path)")
	fmt.Println("  - GET  /api/edgar/fsap-map-stream  (SSE streaming)")
	// ... existing logs ...
	fmt.Println("  - POST /api/edgar/fsap-map  (NEW: FSAP format with source_path)")
	fmt.Println("  - GET  /api/edgar/fsap-map-stream  (SSE streaming)")

	// Use log.Fatal to print error and exit with code 1 if it fails (e.g. port in use)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("[FATAL] Server failed to start: %v\n", err)
		os.Exit(1)
	}
}
