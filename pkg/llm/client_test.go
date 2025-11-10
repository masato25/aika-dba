package llm

import (
	"context"
	"testing"
	"time"

	"github.com/masato25/aika-dba/config"
)

func TestLLMProviders(t *testing.T) {
	// OpenAI 配置測試
	openaiCfg := &config.Config{
		LLM: config.LLMConfig{
			Provider:       "openai",
			Model:          "gpt-3.5-turbo",
			APIKey:         "test-key",
			BaseURL:        "https://api.openai.com/v1",
			TimeoutSeconds: 30,
			MaxTokens:      1024,
			Temperature:    0.7,
			TopP:           0.9,
		},
	}

	// LLamaCpp 配置測試
	llamaCfg := &config.Config{
		LLM: config.LLMConfig{
			Provider:       "llamacpp",
			Model:          "mistral-7b",
			Host:           "localhost",
			Port:           8080,
			TimeoutSeconds: 30,
			MaxTokens:      1024,
			Temperature:    0.7,
			TopP:           0.9,
			TopK:           40,
			StopWords:      []string{"</s>", "\n\n"},
		},
	}

	tests := []struct {
		name     string
		cfg      *config.Config
		prompt   string
		wantErr  bool
		provider string
	}{
		{
			name:     "OpenAI Provider",
			cfg:      openaiCfg,
			prompt:   "Hello, how are you?",
			wantErr:  false,
			provider: "openai",
		},
		{
			name:     "LLamaCpp Provider",
			cfg:      llamaCfg,
			prompt:   "Hello, how are you?",
			wantErr:  false,
			provider: "llamacpp",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.cfg)
			// 為測試設置較短的超時時間
			client.httpClient.Timeout = 2 * time.Second

			_, err := client.GenerateCompletion(ctx, tt.prompt)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateCompletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 檢查客戶端配置是否正確設置
			if client.config.LLM.Provider != tt.provider {
				t.Errorf("Client provider = %v, want %v", client.config.LLM.Provider, tt.provider)
			}

			// 驗證配置參數
			switch tt.provider {
			case "openai":
				if client.config.LLM.APIKey == "" {
					t.Error("OpenAI API key not set")
				}
				if client.config.LLM.BaseURL == "" {
					t.Error("OpenAI base URL not set")
				}
			case "llamacpp":
				if client.config.LLM.Host == "" {
					t.Error("LLamaCpp host not set")
				}
				if client.config.LLM.Port == 0 {
					t.Error("LLamaCpp port not set")
				}
			}
		})
	}
}
