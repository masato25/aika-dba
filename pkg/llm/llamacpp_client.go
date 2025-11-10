package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// LlamaCppRequest represents a request to the llama.cpp server
type LlamaCppRequest struct {
	Prompt        string   `json:"prompt"`
	NPredict      int      `json:"n_predict"`
	Temperature   float64  `json:"temperature"`
	Stop          []string `json:"stop,omitempty"`
	RepeatPenalty float64  `json:"repeat_penalty,omitempty"`
	TopK          int      `json:"top_k,omitempty"`
	TopP          float64  `json:"top_p,omitempty"`
}

// LlamaCppResponse represents a response from the llama.cpp server
type LlamaCppResponse struct {
	Content    string `json:"content"`
	Stop       bool   `json:"stop"`
	Generation struct {
		Prompt      string  `json:"prompt"`
		Temperature float64 `json:"temperature"`
	} `json:"generation_settings"`
	Timings struct {
		PredictedMs float64 `json:"predicted_ms"`
	} `json:"timings"`
}

// generateLocalLlamaCppCompletion generates a completion using local llama.cpp server
func (c *Client) generateLocalLlamaCppCompletion(ctx context.Context, prompt string) (string, error) {
	requestBody := LlamaCppRequest{
		Prompt:        prompt,
		NPredict:      c.config.LLM.MaxTokens,
		Temperature:   c.config.LLM.Temperature,
		Stop:          []string{"</s>", "\n\n"}, // 可以從配置讀取
		RepeatPenalty: 1.1,                      // 可以從配置讀取
		TopK:          40,                       // 可以從配置讀取
		TopP:          c.config.LLM.TopP,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/completion", c.config.LLM.Host, c.config.LLM.Port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response LlamaCppResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// 可以在這裡加入日誌，記錄生成時間和其他統計信息
	if response.Timings.PredictedMs > 0 {
		log.Printf("Generation took %.2fms", response.Timings.PredictedMs)
	}

	return response.Content, nil
}
