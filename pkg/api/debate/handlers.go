package debate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"agentic_valuation/pkg/core/debate"
)

type StartDebateRequest struct {
	Ticker     string `json:"ticker"`
	Company    string `json:"company"`
	FiscalYear string `json:"fiscal_year"`
	Simulation bool   `json:"simulation"`
	Mode       string `json:"mode"` // "automatic" or "interactive"
}

type StartDebateResponse struct {
	DebateID string `json:"debate_id"`
}

type HumanQuestionRequest struct {
	DebateID    string `json:"debate_id"`
	TargetAgent string `json:"target_agent"` // macro, sentiment, fundamentals, skeptic, optimist
	Question    string `json:"question"`
}

type ResumeDebateRequest struct {
	DebateID string `json:"debate_id"`
}

// HandleStartDebate initiates a new background debate
func HandleStartDebate(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartDebateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Company == "" || req.FiscalYear == "" {
		http.Error(w, "Company and FiscalYear are required", http.StatusBadRequest)
		return
	}

	// Convert mode string to DebateMode
	var mode debate.DebateMode
	if req.Mode == "interactive" {
		mode = debate.ModeInteractive
	} else {
		mode = debate.ModeAutomatic
	}

	manager := debate.GetManager()

	// Ensure ticker is populated
	ticker := req.Ticker
	if ticker == "" {
		ticker = req.Company
	}

	id, err := manager.StartDebate(ticker, req.Company, req.FiscalYear, req.Simulation, mode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start debate: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(StartDebateResponse{DebateID: id})
}

// HandleSubmitQuestion allows human to submit a question to an agent
func HandleSubmitQuestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HumanQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.DebateID == "" || req.TargetAgent == "" || req.Question == "" {
		http.Error(w, "debate_id, target_agent, and question are required", http.StatusBadRequest)
		return
	}

	manager := debate.GetManager()
	orch, exists := manager.GetDebate(req.DebateID)
	if !exists {
		http.Error(w, "Debate not found", http.StatusNotFound)
		return
	}

	// Convert string to AgentRole
	targetRole := debate.AgentRole(req.TargetAgent)
	orch.SubmitHumanQuestion(targetRole, req.Question)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
}

// HandleResumeDebate allows human to resume the debate to next phase
func HandleResumeDebate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResumeDebateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.DebateID == "" {
		http.Error(w, "debate_id is required", http.StatusBadRequest)
		return
	}

	manager := debate.GetManager()
	orch, exists := manager.GetDebate(req.DebateID)
	if !exists {
		http.Error(w, "Debate not found", http.StatusNotFound)
		return
	}

	orch.ResumeDebate()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

// HandleStreamDebate provides an SSE stream of debate messages, including history
func HandleStreamDebate(w http.ResponseWriter, r *http.Request) {
	// CORS for EventSource
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	// Extract ID from URL path: /api/debate/stream?id=xyz OR /api/debate/{id}/stream
	// For simplicity with http.HandleFunc, assuming query param or messy path parsing
	// Better approach: use query param ?id=...
	debateID := r.URL.Query().Get("id")
	if debateID == "" {
		// Fallback parse basic path if needed, but Query param is safer for standard http mux
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	manager := debate.GetManager()
	orch, exists := manager.GetDebate(debateID)
	if !exists {
		http.Error(w, "Debate ID not found", http.StatusNotFound)
		return
	}

	// 1. Subscribe to updates
	msgChan, history := orch.Subscribe()
	defer orch.Unsubscribe(msgChan)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 2. Send History immediatey
	for _, msg := range history {
		if err := sendSSE(w, flusher, msg); err != nil {
			return // Client disconnected
		}
	}

	// 3. Stream live updates
	// Heartbeat ticker to keep connection alive
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	notify := r.Context().Done()

	for {
		select {
		case msg, open := <-msgChan:
			if !open {
				// Channel closed means debate finished/unsubscribed
				sendSSEEvent(w, flusher, "status", "completed")
				return
			}
			if err := sendSSE(w, flusher, msg); err != nil {
				return
			}
			// If debate completed, orchestrator sets status. We can check message content or just rely on orchestrator status
			if orch.Status == debate.StatusCompleted {
				sendSSEEvent(w, flusher, "status", "completed")
			}

		case <-ticker.C:
			// Send comment request to keep connection alive
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-notify:
			// Client disconnected
			return
		}
	}
}

// HandleActiveDebates returns a list of active debate IDs
func HandleActiveDebates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	manager := debate.GetManager()
	active := manager.GetActiveDebates()

	json.NewEncoder(w).Encode(map[string][]string{"active_debates": active})
}

// Helper to send a JSON data event
func sendSSE(w http.ResponseWriter, flusher http.Flusher, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
	return nil
}

// Helper to send a typed event
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}
