package phases

import (
	"encoding/json"
	"fmt"
	"os"
)

// Phase1ResultReader Phase 1 結果讀取器
type Phase1ResultReader struct {
	filePath string
}

// Phase1Result Phase 1 的分析結果
type Phase1Result struct {
	Database     string                         `json:"database"`
	DatabaseType string                         `json:"database_type"`
	Timestamp    string                         `json:"timestamp"`
	TablesCount  int                            `json:"tables_count"`
	Tables       map[string]TableAnalysisResult `json:"tables"`
}

// TableAnalysisResult 單個表格的分析結果
type TableAnalysisResult struct {
	Schema      []map[string]interface{} `json:"schema"`
	Constraints map[string]interface{}   `json:"constraints"`
	Indexes     []map[string]interface{} `json:"indexes"`
	Samples     []map[string]interface{} `json:"samples"`
	Stats       map[string]interface{}   `json:"stats"`
}

// NewPhase1ResultReader 創建 Phase 1 結果讀取器
func NewPhase1ResultReader(filePath string) *Phase1ResultReader {
	return &Phase1ResultReader{
		filePath: filePath,
	}
}

// ReadResult 讀取 Phase 1 的分析結果
func (r *Phase1ResultReader) ReadResult() (*Phase1Result, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open phase1 result file: %v", err)
	}
	defer file.Close()

	var result Phase1Result
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode phase1 result: %v", err)
	}

	return &result, nil
}

// GetTableNames 獲取所有表格名稱
func (r *Phase1ResultReader) GetTableNames() ([]string, error) {
	result, err := r.ReadResult()
	if err != nil {
		return nil, err
	}

	tableNames := make([]string, 0, len(result.Tables))
	for tableName := range result.Tables {
		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}

// GetTableAnalysis 獲取特定表格的分析結果
func (r *Phase1ResultReader) GetTableAnalysis(tableName string) (*TableAnalysisResult, error) {
	result, err := r.ReadResult()
	if err != nil {
		return nil, err
	}

	tableResult, exists := result.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table %s not found in phase1 results", tableName)
	}

	return &tableResult, nil
}

// GetTableSummary 獲取表格摘要信息（用於 LLM 分析）
func (r *Phase1ResultReader) GetTableSummary(tableName string) (map[string]interface{}, error) {
	tableResult, err := r.GetTableAnalysis(tableName)
	if err != nil {
		return nil, err
	}

	// 創建表格摘要
	summary := map[string]interface{}{
		"table_name":      tableName,
		"column_count":    len(tableResult.Schema),
		"sample_count":    len(tableResult.Samples),
		"has_constraints": len(tableResult.Constraints) > 0,
		"has_indexes":     len(tableResult.Indexes) > 0,
	}

	// 添加欄位摘要
	columns := make([]map[string]interface{}, 0, len(tableResult.Schema))
	for _, col := range tableResult.Schema {
		column := map[string]interface{}{
			"name":     col["name"],
			"type":     col["type"],
			"nullable": col["nullable"],
		}
		if def, ok := col["default"]; ok && def != nil {
			column["default"] = def
		}
		columns = append(columns, column)
	}
	summary["columns"] = columns

	// 添加約束摘要
	if len(tableResult.Constraints) > 0 {
		constraints := map[string]interface{}{}
		if pks, ok := tableResult.Constraints["primary_keys"]; ok {
			if pkSlice, ok := pks.([]interface{}); ok {
				constraints["primary_keys"] = pkSlice
			}
		}
		if fks, ok := tableResult.Constraints["foreign_keys"]; ok {
			if fkSlice, ok := fks.([]interface{}); ok {
				constraints["foreign_keys_count"] = len(fkSlice)
			}
		}
		if uks, ok := tableResult.Constraints["unique_keys"]; ok {
			if ukSlice, ok := uks.([]interface{}); ok {
				constraints["unique_keys_count"] = len(ukSlice)
			}
		}
		summary["constraints"] = constraints
	}

	// 添加索引摘要
	if len(tableResult.Indexes) > 0 {
		indexes := make([]map[string]interface{}, 0, len(tableResult.Indexes))
		for _, idx := range tableResult.Indexes {
			index := map[string]interface{}{
				"name":   idx["name"],
				"unique": idx["is_unique"],
			}
			if cols, ok := idx["columns"]; ok {
				index["columns"] = cols
			}
			indexes = append(indexes, index)
		}
		summary["indexes"] = indexes
	}

	// 添加統計信息
	if len(tableResult.Stats) > 0 {
		summary["stats"] = tableResult.Stats
	}

	return summary, nil
}

// GetDatabaseOverview 獲取資料庫總覽
func (r *Phase1ResultReader) GetDatabaseOverview() (map[string]interface{}, error) {
	result, err := r.ReadResult()
	if err != nil {
		return nil, err
	}

	overview := map[string]interface{}{
		"database":      result.Database,
		"database_type": result.DatabaseType,
		"tables_count":  result.TablesCount,
		"timestamp":     result.Timestamp,
	}

	// 統計各表格的資訊
	totalColumns := 0
	totalSamples := 0
	tablesWithConstraints := 0
	tablesWithIndexes := 0

	for _, tableResult := range result.Tables {
		totalColumns += len(tableResult.Schema)
		totalSamples += len(tableResult.Samples)

		if len(tableResult.Constraints) > 0 {
			tablesWithConstraints++
		}

		if len(tableResult.Indexes) > 0 {
			tablesWithIndexes++
		}
	}

	overview["total_columns"] = totalColumns
	overview["total_samples"] = totalSamples
	overview["tables_with_constraints"] = tablesWithConstraints
	overview["tables_with_indexes"] = tablesWithIndexes

	return overview, nil
}
