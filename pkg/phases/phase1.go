package phases

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// Phase1Runner Phase 1 執行器
type Phase1Runner struct {
	analyzer     *analyzer.DatabaseAnalyzer
	config       *config.Config
	knowledgeMgr *vectorstore.KnowledgeManager
}

// NewPhase1Runner 創建 Phase 1 執行器
func NewPhase1Runner(dbAnalyzer *analyzer.DatabaseAnalyzer, cfg *config.Config) (*Phase1Runner, error) {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		return nil, err
	}

	return &Phase1Runner{
		analyzer:     dbAnalyzer,
		config:       cfg,
		knowledgeMgr: knowledgeMgr,
	}, nil
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
	ignoredTables := make([]string, 0)

	for _, tableName := range tables {
		log.Printf("Analyzing table: %s", tableName)

		// 使用分析器的 AnalyzeTable 方法
		analysis, err := p.analyzer.AnalyzeTable(tableName, p.config.Schema.MaxSamples)
		if err != nil {
			log.Printf("Warning: Failed to analyze table %s: %v", tableName, err)
			continue
		}

		// 檢查是否為空表格
		if p.isEmptyTable(analysis) {
			log.Printf("Table %s is empty (0 records), marking as default ignored", tableName)
			ignoredTables = append(ignoredTables, tableName)
			continue // 不將空表格加入分析結果
		}

		tableAnalyses[tableName] = analysis
	}

	// 創建輸出
	output := map[string]interface{}{
		"database":       p.config.Database.DBName,
		"database_type":  p.config.Database.Type,
		"timestamp":      time.Now(),
		"tables_count":   len(tables),
		"analyzed_count": len(tableAnalyses),
		"ignored_count":  len(ignoredTables),
		"tables":         tableAnalyses,
		"ignored_tables": ignoredTables,
	}

	// 寫入文件
	if err := p.writeOutput(output, "knowledge/phase1_analysis.json"); err != nil {
		return err
	}

	// 創建摘要版本用於向量存儲（避免存儲龐大的詳細數據）
	summaryOutput := map[string]interface{}{
		"phase":          "phase1",
		"description":    "Database schema and statistical analysis summary",
		"database":       p.config.Database.DBName,
		"database_type":  p.config.Database.Type,
		"timestamp":      time.Now(),
		"tables_count":   len(tables),
		"analyzed_count": len(tableAnalyses),
		"ignored_count":  len(ignoredTables),
		"summary": map[string]interface{}{
			"total_tables_analyzed": len(tableAnalyses),
			"total_tables_ignored":  len(ignoredTables),
			"database_overview":     p.generateDatabaseOverview(tableAnalyses),
			"key_insights":          p.extractKeyInsights(tableAnalyses),
		},
	}

	// 將摘要知識存儲到向量數據庫
	if err := p.knowledgeMgr.StorePhaseKnowledge("phase1", summaryOutput); err != nil {
		log.Printf("Warning: Failed to store phase1 knowledge in vector store: %v", err)
		// 不返回錯誤，因為 JSON 文件已經寫入成功
	} else {
		log.Printf("Phase 1 knowledge summary stored in vector database")
	}

	return nil
}

// isEmptyTable 檢查表格是否為空（沒有任何記錄）
func (p *Phase1Runner) isEmptyTable(analysis map[string]interface{}) bool {
	// 從分析結果中檢查 row_count
	if stats, ok := analysis["stats"].(map[string]interface{}); ok {
		if rowCount, ok := stats["row_count"]; ok {
			switch v := rowCount.(type) {
			case float64:
				return v == 0
			case int:
				return v == 0
			case int64:
				return v == 0
			}
		}
	}
	return false
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
	log.Printf("Total tables: %d, Analyzed: %d, Ignored (empty): %d",
		len(data.(map[string]interface{})["tables"].(map[string]interface{}))+len(data.(map[string]interface{})["ignored_tables"].([]string)),
		len(data.(map[string]interface{})["tables"].(map[string]interface{})),
		len(data.(map[string]interface{})["ignored_tables"].([]string)))

	return nil
}

// Close 關閉 Phase 1 執行器
func (p *Phase1Runner) Close() error {
	if p.knowledgeMgr != nil {
		return p.knowledgeMgr.Close()
	}
	return nil
}

// generateDatabaseOverview 生成數據庫總覽
func (p *Phase1Runner) generateDatabaseOverview(tableAnalyses map[string]interface{}) map[string]interface{} {
	totalRows := 0
	totalColumns := 0
	tableTypes := make(map[string]int)

	for _, analysis := range tableAnalyses {
		if analysisMap, ok := analysis.(map[string]interface{}); ok {
			// 統計總行數
			if stats, ok := analysisMap["stats"].(map[string]interface{}); ok {
				if rowCount, ok := stats["row_count"]; ok {
					switch v := rowCount.(type) {
					case float64:
						totalRows += int(v)
					case int:
						totalRows += v
					case int64:
						totalRows += int(v)
					}
				}
			}

			// 統計總列數
			if columns, ok := analysisMap["columns"].([]interface{}); ok {
				totalColumns += len(columns)
			}

			// 統計表格類型（基於表格名稱模式）
			if tableInfo, ok := analysisMap["table_info"].(map[string]interface{}); ok {
				if tableName, ok := tableInfo["name"].(string); ok {
					tableType := p.categorizeTableType(tableName)
					tableTypes[tableType]++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_tables":    len(tableAnalyses),
		"total_rows":      totalRows,
		"total_columns":   totalColumns,
		"average_columns": float64(totalColumns) / float64(len(tableAnalyses)),
		"table_types":     tableTypes,
	}
}

// extractKeyInsights 提取關鍵洞察
func (p *Phase1Runner) extractKeyInsights(tableAnalyses map[string]interface{}) map[string]interface{} {
	largestTables := make([]map[string]interface{}, 0)
	smallestTables := make([]map[string]interface{}, 0)

	for tableName, analysis := range tableAnalyses {
		if analysisMap, ok := analysis.(map[string]interface{}); ok {
			if stats, ok := analysisMap["stats"].(map[string]interface{}); ok {
				if rowCount, ok := stats["row_count"]; ok {
					var rows int
					switch v := rowCount.(type) {
					case float64:
						rows = int(v)
					case int:
						rows = v
					case int64:
						rows = int(v)
					}

					tableInfo := map[string]interface{}{
						"name":      tableName,
						"row_count": rows,
					}

					// 收集最大的表格
					if len(largestTables) < 3 {
						largestTables = append(largestTables, tableInfo)
					} else {
						// 簡單的排序，保持最大的3個
						for i, t := range largestTables {
							if existingRows, ok := t["row_count"].(int); ok && rows > existingRows {
								largestTables[i] = tableInfo
								break
							}
						}
					}

					// 收集最小的表格
					if len(smallestTables) < 3 {
						smallestTables = append(smallestTables, tableInfo)
					} else {
						// 簡單的排序，保持最小的3個
						for i, t := range smallestTables {
							if existingRows, ok := t["row_count"].(int); ok && rows < existingRows {
								smallestTables[i] = tableInfo
								break
							}
						}
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"largest_tables":  largestTables,
		"smallest_tables": smallestTables,
		"total_tables":    len(tableAnalyses),
		"analysis_type":   "schema_statistics",
	}
}

// categorizeTableType 根據表格名稱分類表格類型
func (p *Phase1Runner) categorizeTableType(tableName string) string {
	tableName = strings.ToLower(tableName)

	switch {
	case strings.Contains(tableName, "user") || strings.Contains(tableName, "customer"):
		return "user_management"
	case strings.Contains(tableName, "order") || strings.Contains(tableName, "purchase"):
		return "transaction"
	case strings.Contains(tableName, "product") || strings.Contains(tableName, "item"):
		return "product_catalog"
	case strings.Contains(tableName, "payment") || strings.Contains(tableName, "billing"):
		return "financial"
	case strings.Contains(tableName, "log") || strings.Contains(tableName, "audit"):
		return "logging"
	case strings.Contains(tableName, "config") || strings.Contains(tableName, "setting"):
		return "configuration"
	default:
		return "general"
	}
}
