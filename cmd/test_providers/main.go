package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
)

func main() {
	// 測試 OpenAI 配置
	openaiCfg := &config.MainConfig{
		LLM: &config.LLMConfig{
			Provider:       "openai",
			Model:          "gpt-3.5-turbo",
			APIKey:         os.Getenv("OPENAI_API_KEY"),
			TimeoutSeconds: 30,
			MaxTokens:      1024,
			Temperature:    0.7,
			TopP:           0.9,
		},
	}

	// 測試 LlamaCpp 配置
	llamaCfg := &config.MainConfig{
		LLM: &config.LLMConfig{
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

	ctx := context.Background()
	testPrompt := "Write a short poem about coding."

	// 測試 OpenAI
	fmt.Println("Testing OpenAI:")
	openaiClient := llm.NewClient(openaiCfg)
	if resp, err := openaiClient.GenerateCompletion(ctx, testPrompt); err != nil {
		log.Printf("OpenAI error: %v\n", err)
	} else {
		fmt.Printf("OpenAI response:\n%s\n\n", resp)
	}

	// 測試 LlamaCpp
	fmt.Println("Testing LlamaCpp:")
	llamaClient := llm.NewClient(llamaCfg)
	if resp, err := llamaClient.GenerateCompletion(ctx, testPrompt); err != nil {
		log.Printf("LlamaCpp error: %v\n", err)
	} else {
		fmt.Printf("LlamaCpp response:\n%s\n\n", resp)
	}
}
