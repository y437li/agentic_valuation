package debate

import (
	"context"
	"encoding/json"
	"fmt"

	"agentic_valuation/pkg/core/store"
)

// DebateRepo handles persistence for debate sessions and messages
type DebateRepo struct{}

// NewDebateRepo creates a new instance of DebateRepo
func NewDebateRepo() *DebateRepo {
	return &DebateRepo{}
}

// CreateDebate initializes a new debate session record
func (r *DebateRepo) CreateDebate(ctx context.Context, id, company, fiscalYear string) error {
	pool := store.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT INTO debates (id, company, fiscal_year, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`
	_, err := pool.Exec(ctx, query, id, company, fiscalYear, StatusIdle)
	if err != nil {
		return fmt.Errorf("failed to create debate record: %w", err)
	}
	return nil
}

// UpdateStatus updates the status of a debate
func (r *DebateRepo) UpdateStatus(ctx context.Context, id string, status DebateStatus) error {
	pool := store.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		UPDATE debates 
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update debate status: %w", err)
	}
	return nil
}

// SaveFinalReport persists the final debate report
func (r *DebateRepo) SaveFinalReport(ctx context.Context, report *FinalDebateReport) error {
	pool := store.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialized")
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal final report: %w", err)
	}

	query := `
		UPDATE debates 
		SET report = $2, status = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query, report.DebateID, reportJSON, StatusCompleted)
	if err != nil {
		return fmt.Errorf("failed to save final report: %w", err)
	}
	return nil
}

// AddMessage appends a new message to the debate transcript
func (r *DebateRepo) AddMessage(ctx context.Context, debateID string, msg DebateMessage) error {
	pool := store.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialized")
	}

	refsJSON, err := json.Marshal(msg.References)
	if err != nil {
		return fmt.Errorf("failed to marshal references: %w", err)
	}

	query := `
		INSERT INTO debate_messages (debate_id, round_index, agent_role, agent_name, content, references, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = pool.Exec(ctx, query, debateID, msg.Round, msg.AgentRole, msg.AgentName, msg.Content, refsJSON, msg.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to add debate message: %w", err)
	}
	return nil
}

// GetDebateHistory retrieves the full transcript for a debate
func (r *DebateRepo) GetDebateHistory(ctx context.Context, debateID string) ([]DebateMessage, error) {
	pool := store.GetPool()
	if pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT round_index, agent_role, agent_name, content, "references", timestamp
		FROM debate_messages
		WHERE debate_id = $1
		ORDER BY id ASC
	`
	rows, err := pool.Query(ctx, query, debateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query debate history: %w", err)
	}
	defer rows.Close()

	var history []DebateMessage
	for rows.Next() {
		var msg DebateMessage
		var refsJSON []byte
		var roleStr string

		if err := rows.Scan(&msg.Round, &roleStr, &msg.AgentName, &msg.Content, &refsJSON, &msg.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		msg.AgentRole = AgentRole(roleStr)
		msg.ID = fmt.Sprintf("%s-%d", debateID, msg.Round)

		if len(refsJSON) > 0 {
			if err := json.Unmarshal(refsJSON, &msg.References); err != nil {
				return nil, fmt.Errorf("failed to unmarshal references: %w", err)
			}
		}

		history = append(history, msg)
	}

	return history, nil
}
