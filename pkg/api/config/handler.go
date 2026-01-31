package config

import (
	"encoding/json"
	"fmt"
	"net/http"

	"agentic_valuation/pkg/core/agent"
)

type Response struct {
	ActiveProvider string   `json:"active_provider"`
	Available      []string `json:"available"`
}

type SwitchRequest struct {
	Provider string `json:"provider"`
}

// Handler holds dependencies for config endpoints
type Handler struct {
	AgentMgr *agent.Manager
}

// NewHandler creates a new config handler
func NewHandler(agentMgr *agent.Manager) *Handler {
	return &Handler{
		AgentMgr: agentMgr,
	}
}

func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for local dev
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	resp := Response{
		ActiveProvider: h.AgentMgr.GetActiveProvider(),
		Available:      []string{"openai", "gemini", "deepseek", "qwen", "kimi", "doubao"},
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleSwitch(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req SwitchRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.AgentMgr.SetGlobalProvider(req.Provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Success: Switched to %s", req.Provider)
}
