package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

func main() {
	// 載入環境變數
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("測試向量嵌入生成器: %s\n", cfg.VectorStore.EmbedderType)

	// 創建知識管理器來測試嵌入生成器
	km, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create knowledge manager: %v", err)
	}
	defer km.Close()

	// 測試嵌入生成
	testText := "這是一個測試文本，用於驗證嵌入生成功能"
	fmt.Printf("測試文本: %s\n", testText)

	// 我們無法直接訪問嵌入生成器，但可以通過創建一個簡單的測試
	// 這裡我們創建一個新的嵌入生成器來測試

	var embedder vectorstore.Embedder
	switch cfg.VectorStore.EmbedderType {
	case "openai":
		apiKey := cfg.LLM.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		baseURL := cfg.LLM.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := cfg.LLM.Model
		if model == "" {
			model = os.Getenv("LLM_MODEL")
		}
		if model == "" {
			model = "text-embedding-3-small"
		}

		if apiKey == "" {
			fmt.Println("❌ OPENAI_API_KEY 未設定，無法測試 OpenAI 嵌入")
			fmt.Println("請在 .env 文件中設定 OPENAI_API_KEY")
			return
		}

		embedder = vectorstore.NewOpenAIEmbedder(apiKey, baseURL, model)
		fmt.Printf("✅ 使用 OpenAI 嵌入: %s\n", model)

	case "qwen":
		embedder = vectorstore.NewQwenEmbedder("", cfg.VectorStore.EmbeddingDimension)
		fmt.Println("✅ 使用 Qwen 嵌入")

	case "simple":
		embedder = vectorstore.NewSimpleHashEmbedder(cfg.VectorStore.EmbeddingDimension)
		fmt.Println("✅ 使用簡單哈希嵌入")

	default:
		fmt.Printf("❌ 不支援的嵌入類型: %s\n", cfg.VectorStore.EmbedderType)
		return
	}

	// 生成嵌入
	vector, err := embedder.GenerateEmbedding(testText)
	if err != nil {
		log.Fatalf("Failed to generate embedding: %v", err)
	}

	fmt.Printf("✅ 嵌入生成成功!\n")
	fmt.Printf("向量維度: %d\n", len(vector))
	fmt.Printf("前5個值: %v\n", vector[:min(5, len(vector))])

	// 測試統計
	stats, err := km.GetKnowledgeStats()
	if err != nil {
		log.Printf("Warning: Failed to get knowledge stats: %v", err)
	} else {
		fmt.Printf("知識庫統計: %+v\n", stats)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}