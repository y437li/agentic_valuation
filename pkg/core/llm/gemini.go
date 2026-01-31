package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// GeminiProvider implements the Provider interface for Google's Gemini models.
type GeminiProvider struct {
	Model string // e.g. "gemini-2.0-flash-exp"
}

// Ensure interface compliance
var _ Provider = (*GeminiProvider)(nil)

// GenerateResponse sends a generateContent request to the Gemini API using the official GenAI SDK.
func (p *GeminiProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	// Determine model
	model := p.Model
	if model == "" {
		model = "gemini-2.0-flash-exp"
	}
	// Allow override from options
	if val, ok := options["model"].(string); ok && val != "" {
		model = val
	}

	// Initialize Client
	// We use the simpler client initialization if possible, or configuration-based.
	// Based on standard usage of this alpha SDK:
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create GenAI client: %w", err)
	}

	// Prepare Config
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.1)), // SDK expects *float32
	}

	// Check for JSON mode
	// 1. From options
	if val, ok := options["response_format"].(map[string]interface{}); ok {
		if val["type"] == "json_object" {
			config.ResponseMIMEType = "application/json"
		}
	} else if strings.Contains(strings.ToLower(systemPrompt), "json") || strings.Contains(strings.ToLower(prompt), "json") {
		// Heuristic
		config.ResponseMIMEType = "application/json"
	}

	// Add System Instruction if present
	if systemPrompt != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		}
	}

	// Handle Google Search Grounding
	if val, ok := options["google_search"].(bool); ok && val {
		config.Tools = []*genai.Tool{
			{GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{}},
		}
	}

	// Exec Generation
	result, err := client.Models.GenerateContent(
		ctx,
		model,
		genai.Text(prompt),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("gemini generation failed: %w", err)
	}

	// Return text with citations
	text := result.Text()

	// Extract grounding metadata if present
	if len(result.Candidates) > 0 {
		cand := result.Candidates[0]
		if cand.GroundingMetadata != nil && len(cand.GroundingMetadata.GroundingChunks) > 0 {
			var citations []string
			for _, chunk := range cand.GroundingMetadata.GroundingChunks {
				if chunk.Web != nil {
					citations = append(citations, fmt.Sprintf("[%s](%s)", chunk.Web.Title, chunk.Web.URI))
				}
			}
			if len(citations) > 0 {
				text = fmt.Sprintf("%s\n\n**Sources:**\n%s", text, strings.Join(citations, "\n"))
			}
		}
	}

	return text, nil
}

func (p *GeminiProvider) AdaptInstructions(raw string) string {
	return raw
}
