package phases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/mcp"
)

// Phase2Runner Phase 2 執行器
type Phase2Runner struct {
	config    *config.Config
	db        *sql.DB
	reader    *Phase1ResultReader
	analyzer  *TableAnalysisOrchestrator
	mcpServer *mcp.MCPServer
}

// NewPhase2Runner 創建 Phase 2 執行器
func NewPhase2Runner(cfg *config.Config, db *sql.DB) *Phase2Runner {
	// 創建 phase1 結果讀取器
	reader := NewPhase1ResultReader("knowledge/phase1_analysis.json")

	// 創建 MCP 服務器
	mcpServer := mcp.NewMCPServer(db)

	// 創建表格分析協調器
	analyzer := NewTableAnalysisOrchestrator(cfg, reader, mcpServer)

	return &Phase2Runner{
		config:    cfg,
		db:        db,
		reader:    reader,
		analyzer:  analyzer,
		mcpServer: mcpServer,
	}
}

// Run 執行 Phase 2 AI 分析
func (p *Phase2Runner) Run() error {
	log.Println("=== Starting Phase 2: AI Analysis ===")

	// 檢查 LLM 配置
	log.Printf("LLM Configuration:")
	log.Printf("  Provider: %s", p.config.LLM.Provider)
	log.Printf("  Model: %s", p.config.LLM.Model)
	log.Printf("  Host: %s", p.config.LLM.Host)
	log.Printf("  Port: %d", p.config.LLM.Port)
	log.Printf("  Base URL: %s", p.config.LLM.BaseURL)

	// 初始化分析任務
	if err := p.analyzer.InitializeTasks(); err != nil {
		return fmt.Errorf("failed to initialize analysis tasks: %v", err)
	}

	// 執行分析
	ctx := context.Background()
	if err := p.runAnalysis(ctx); err != nil {
		return fmt.Errorf("failed to run analysis: %v", err)
	}

	// 保存結果
	if err := p.saveResults(); err != nil {
		return fmt.Errorf("failed to save results: %v", err)
	}

	log.Printf("Phase 2 AI analysis completed successfully")
	return nil
}

// runAnalysis 執行分析流程
func (p *Phase2Runner) runAnalysis(ctx context.Context) error {
	log.Println("Starting table analysis process...")

	for {
		// 獲取下一個任務
		task := p.analyzer.GetNextTask()
		if task == nil {
			// 所有任務都完成了
			break
		}

		log.Printf("Processing table: %s", task.TableName)

		// 開始任務
		p.analyzer.StartTask(task)

		// 分析表格
		result, err := p.analyzer.AnalyzeTable(ctx, task)
		if err != nil {
			log.Printf("Failed to analyze table %s: %v", task.TableName, err)
			p.analyzer.FailTask(task, err)
			continue
		}

		// 完成任務
		p.analyzer.CompleteTask(task, result)

		// 顯示進度
		progress := p.analyzer.GetProgress()
		log.Printf("Progress: %.1f%% (%d/%d completed)",
			progress["percentage"].(float64),
			progress["completed"].(int),
			progress["total"].(int))
	}

	log.Println("All table analyses completed")
	return nil
}

// saveResults 保存分析結果
func (p *Phase2Runner) saveResults() error {
	results := p.analyzer.GetResults()

	// 創建輸出結構
	output := map[string]interface{}{
		"phase":         "phase2",
		"description":   "AI-powered database analysis",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"llm_config": map[string]interface{}{
			"provider": p.config.LLM.Provider,
			"model":    p.config.LLM.Model,
			"host":     p.config.LLM.Host,
			"port":     p.config.LLM.Port,
		},
		"analysis_results": results,
		"summary":          p.generateSummary(results),
	}

	// 寫入文件
	return p.writeOutput(output, "knowledge/phase2_analysis.json")
}

// generateSummary 生成總結
func (p *Phase2Runner) generateSummary(results map[string]*LLMAnalysisResult) map[string]interface{} {
	totalTables := len(results)
	totalRecommendations := 0
	totalIssues := 0
	totalInsights := 0

	for _, result := range results {
		totalRecommendations += len(result.Recommendations)
		totalIssues += len(result.Issues)
		totalInsights += len(result.Insights)
	}

	return map[string]interface{}{
		"total_tables_analyzed":             totalTables,
		"total_recommendations":             totalRecommendations,
		"total_issues_found":                totalIssues,
		"total_insights":                    totalInsights,
		"average_recommendations_per_table": float64(totalRecommendations) / float64(totalTables),
		"average_issues_per_table":          float64(totalIssues) / float64(totalTables),
		"average_insights_per_table":        float64(totalInsights) / float64(totalTables),
	}
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

	log.Printf("Phase 2 AI analysis results saved to %s", filename)
	return nil
}
