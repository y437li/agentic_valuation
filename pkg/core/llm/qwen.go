package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type QwenProvider struct{}

func (p *QwenProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	// 1. Get API Key from options or env
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if val, ok := options["api_key"].(string); ok && val != "" {
		apiKey = val
	}
	// Fallback to QWEN_API_KEY if DASHSCOPE_API_KEY is not set
	if apiKey == "" {
		apiKey = os.Getenv("QWEN_API_KEY")
	}

	if apiKey == "" {
		return "", fmt.Errorf("QWEN_API_KEY_MISSING: Please set DASHSCOPE_API_KEY or QWEN_API_KEY")
	}

	// 2. Get Model
	model := "qwen-max"
	if val, ok := options["model"].(string); ok && val != "" {
		model = val
	}

	// 3. Construct Request Body (Native DashScope API format)
	// See: https://help.aliyun.com/document_detail/2712532.html
	reqBody := map[string]interface{}{
		"model": model,
		"input": map[string]interface{}{
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": prompt},
			},
		},
		"parameters": map[string]interface{}{
			"result_format": "message",
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal qwen request: %w", err)
	}

	// 4. Create HTTP Request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 5. Execute Request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qwen api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("qwen api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 6. Parse Response
	// Response structure:
	// {
	//   "output": {
	//     "choices": [
	//       {
	//         "message": {
	//           "content": "..."
	//         }
	//       }
	//     ]
	//   }
	// }
	var result struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			// Compatibility for some DashScope endpoints that return 'text' directly in output
			Text string `json:"text"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode qwen response: %w", err)
	}

	if result.Code != "" {
		return "", fmt.Errorf("qwen api error: %s - %s", result.Code, result.Message)
	}

	// Try extracting content from choices first (chat format)
	if len(result.Output.Choices) > 0 {
		return result.Output.Choices[0].Message.Content, nil
	}

	// Fallback for text completion format
	if result.Output.Text != "" {
		return result.Output.Text, nil
	}

	return "", fmt.Errorf("empty response from qwen api")
}

func (p *QwenProvider) AdaptInstructions(raw string) string {
	return raw
}
