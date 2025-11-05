package phases

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
)

// Phase1Runner Phase 1 執行器
type Phase1Runner struct {
	analyzer *analyzer.DatabaseAnalyzer
	config   *config.Config
}

// NewPhase1Runner 創建 Phase 1 執行器
func NewPhase1Runner(dbAnalyzer *analyzer.DatabaseAnalyzer, cfg *config.Config) *Phase1Runner {
	return &Phase1Runner{
		analyzer: dbAnalyzer,
		config:   cfg,
	}
}

// Run 執行 Phase 1 統計分析
func (p *Phase1Runner) Run() error {
	log.Println("=== Starting Phase 1: Statistical Analysis ===")

	// 獲取所有表格
	tables, err := p.analyzer.GetAllTables()
	if err != nil {
		return err
	}

	log.Printf("Found %d tables in database", len(tables))

	// 分析每個表格
	tableAnalyses := make(map[string]interface{})
	for _, tableName := range tables {
		log.Printf("Analyzing table: %s", tableName)

		// 使用分析器的 AnalyzeTable 方法
		analysis, err := p.analyzer.AnalyzeTable(tableName, p.config.Schema.MaxSamples)
		if err != nil {
			log.Printf("Warning: Failed to analyze table %s: %v", tableName, err)
			continue
		}

		tableAnalyses[tableName] = analysis
	}

	// 創建輸出
	output := map[string]interface{}{
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"tables_count":  len(tables),
		"tables":        tableAnalyses,
	}

	// 寫入文件
	return p.writeOutput(output, "knowledge/phase1_analysis.json")
}

// writeOutput 寫入輸出到文件
func (p *Phase1Runner) writeOutput(data interface{}, filename string) error {
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

	log.Printf("Phase 1 analysis completed. Results saved to %s", filename)
	log.Printf("Analyzed %d tables with schema and sample data", len(data.(map[string]interface{})["tables"].(map[string]interface{})))

	return nil
}
