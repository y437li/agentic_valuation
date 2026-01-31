package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type DeepSeekProvider struct{}

// DeepSeekRequest mirrors the structure provided in the user's example
type DeepSeekRequest struct {
	Messages         []Message      `json:"messages"`
	Model            string         `json:"model"`
	Thinking         *ThinkingParam `json:"thinking,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty"`
	MaxTokens        int            `json:"max_tokens"`
	PresencePenalty  float64        `json:"presence_penalty"`
	ResponseFormat   ResponseFormat `json:"response_format"`
	Stop             interface{}    `json:"stop"`
	Stream           bool           `json:"stream"`
	StreamOptions    interface{}    `json:"stream_options"`
	Temperature      float64        `json:"temperature"`
	TopP             float64        `json:"top_p"`
	Tools            interface{}    `json:"tools"`
	ToolChoice       string         `json:"tool_choice"`
	LogProbs         bool           `json:"logprobs"`
	TopLogProbs      interface{}    `json:"top_logprobs"`
}

type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type ThinkingParam struct {
	Type string `json:"type"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *DeepSeekProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if val, ok := options["api_key"].(string); ok && val != "" {
		apiKey = val
	}
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY_MISSING: Please set DEEPSEEK_API_KEY env var")
	}

	model := "deepseek-chat"
	if val, ok := options["model"].(string); ok && val != "" {
		model = val
	}

	url := "https://api.deepseek.com/chat/completions"

	reqBody := DeepSeekRequest{
		Messages: []Message{
			{Content: systemPrompt, Role: "system"},
			{Content: prompt, Role: "user"},
		},
		Model: model,
		Thinking: &ThinkingParam{
			Type: "disabled", // Default as per example
		},
		FrequencyPenalty: 0,
		MaxTokens:        4096,
		PresencePenalty:  0,
		ResponseFormat: ResponseFormat{
			Type: "text",
		},
		Stop:        nil,
		Stream:      false,
		Temperature: 1.0,
		TopP:        1.0,
		ToolChoice:  "none",
		LogProbs:    false,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("DEEPSEEK_MARSHAL_ERROR: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("DEEPSEEK_REQ_CREATE_ERROR: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DEEPSEEK_API_CALL_ERROR: %v", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("DEEPSEEK_READ_BODY_ERROR: %v", err)
	}

	if res.StatusCode != 200 {
		return "", fmt.Errorf("DEEPSEEK_API_ERROR: status=%d found=%s", res.StatusCode, string(body))
	}

	var response DeepSeekResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("DEEPSEEK_UNMARSHAL_ERROR: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("DEEPSEEK_NO_CHOICES: %s", string(body))
	}

	return response.Choices[0].Message.Content, nil
}

func (p *DeepSeekProvider) AdaptInstructions(raw string) string {
	return raw
}
