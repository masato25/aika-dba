package phases

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/masato25/aika-dba/config"
)

// Phase2Runner Phase 2 執行器
type Phase2Runner struct {
	config *config.Config
}

// NewPhase2Runner 創建 Phase 2 執行器
func NewPhase2Runner(cfg *config.Config) *Phase2Runner {
	return &Phase2Runner{
		config: cfg,
	}
}

// Run 執行 Phase 2 AI 分析
func (p *Phase2Runner) Run() error {
	log.Println("=== Starting Phase 2: AI Analysis ===")

	// 簡化版本：只是顯示一個消息
	log.Println("Phase 2 functionality is not yet implemented")
	log.Println("This would perform AI-powered database analysis")

	// 檢查 LLM 配置
	log.Printf("LLM Configuration:")
	log.Printf("  Provider: %s", p.config.LLM.Provider)
	log.Printf("  Model: %s", p.config.LLM.Model)
	log.Printf("  Host: %s", p.config.LLM.Host)
	log.Printf("  Port: %d", p.config.LLM.Port)
	log.Printf("  Base URL: %s", p.config.LLM.BaseURL)

	// 創建一個示例輸出
	output := map[string]interface{}{
		"message":  "Phase 2 AI analysis placeholder",
		"status":   "coming_soon",
		"database": p.config.Database.DBName,
		"llm_config": map[string]interface{}{
			"provider": p.config.LLM.Provider,
			"model":    p.config.LLM.Model,
			"host":     p.config.LLM.Host,
			"port":     p.config.LLM.Port,
		},
		"timestamp": time.Now(),
	}

	// 寫入文件
	return p.writeOutput(output, "phase2_analysis.json")
}

// writeOutput 寫入輸出到文件
func (p *Phase2Runner) writeOutput(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	log.Printf("Phase 2 AI analysis completed. Results saved to %s", filename)
	return nil
}