package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
)

func main() {
	// 解析命令行參數
	configFile := flag.String("config", "config.yaml", "主配置文件路徑")
	modelConfig := flag.String("model", "", "覆蓋使用的模型配置文件（可選）")
	prompt := flag.String("prompt", "用繁體中文回答：請介紹一下資料庫正規化的概念。", "要測試的提示詞")
	flag.Parse()

	// 確保配置文件路徑是絕對路徑
	absConfigPath, err := filepath.Abs(*configFile)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}

	// 載入配置
	cfg, err := config.LoadConfig(absConfigPath)
	if err != nil {
		log.Fatalf("Error loading config from %s: %v", absConfigPath, err)
	}

	// 如果指定了模型配置，覆蓋主配置中的設置
	if *modelConfig != "" {
		cfg.ModelConfig = *modelConfig
		// 重新載入配置以應用新的模型配置
		cfg, err = config.LoadConfig(absConfigPath)
		if err != nil {
			log.Fatalf("Error reloading config: %v", err)
		}
	}

	// 列印當前使用的配置
	fmt.Printf("Using LLM Provider: %s\n", cfg.LLM.Provider)
	fmt.Printf("Model: %s\n", cfg.LLM.Model)
	if cfg.LLM.Provider == "ollama" || cfg.LLM.Provider == "llamacpp" || cfg.LLM.Provider == "local" {
		fmt.Printf("Server: http://%s:%d\n", cfg.LLM.Host, cfg.LLM.Port)
	}

	// 創建 LLM 客戶端
	client := llm.NewClient(cfg)

	// 生成回應
	ctx := context.Background()
	fmt.Printf("\nSending prompt: %s\n", *prompt)
	fmt.Println("\nWaiting for response...")

	response, err := client.GenerateCompletion(ctx, *prompt)
	if err != nil {
		log.Fatalf("Error generating completion: %v", err)
	}

	// 輸出結果
	fmt.Printf("\nResponse from %s (%s):\n%s\n", cfg.LLM.Provider, cfg.LLM.Model, response)
}
