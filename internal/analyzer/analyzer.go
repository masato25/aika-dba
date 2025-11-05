package analyzer

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/masato25/aika-dba/internal/schema"
)

// DatabaseAnalyzer 資料庫分析器
type DatabaseAnalyzer struct {
	db     *sql.DB
	dbType string
}

// NewDatabaseAnalyzer 創建資料庫分析器
func NewDatabaseAnalyzer(db *sql.DB, dbType string) *DatabaseAnalyzer {
	return &DatabaseAnalyzer{
		db:     db,
		dbType: dbType,
	}
}

// AnalysisResult 分析結果
type AnalysisResult struct {
	DatabaseName  string          `json:"database_name"`
	AnalysisTime  time.Time       `json:"analysis_time"`
	TableAnalyses []TableAnalysis `json:"table_analyses"`
	Summary       DatabaseSummary `json:"summary"`
}

// TableAnalysis 表格分析結果
type TableAnalysis struct {
	TableName      string           `json:"table_name"`
	RecordCount    int64            `json:"record_count"`
	ColumnAnalyses []ColumnAnalysis `json:"column_analyses"`
	Summary        TableSummary     `json:"summary"`
}

// ColumnAnalysis 欄位分析結果
type ColumnAnalysis struct {
	ColumnName   string      `json:"column_name"`
	ColumnType   string      `json:"column_type"`
	TotalCount   int64       `json:"total_count"`
	NotNullCount int64       `json:"not_null_count"`
	NullCount    int64       `json:"null_count"`
	NullRatio    float64     `json:"null_ratio"`
	UniqueCount  int64       `json:"unique_count"`
	MinValue     interface{} `json:"min_value,omitempty"`
	MaxValue     interface{} `json:"max_value,omitempty"`
	AvgValue     interface{} `json:"avg_value,omitempty"`
	SampleValues []string    `json:"sample_values"`
}

// TableSummary 表格摘要
type TableSummary struct {
	TotalColumns     int     `json:"total_columns"`
	PrimaryKeys      int     `json:"primary_keys"`
	ForeignKeys      int     `json:"foreign_keys"`
	NullableRatio    float64 `json:"nullable_ratio"`
	DataCompleteness float64 `json:"data_completeness"`
}

// DatabaseSummary 資料庫摘要
type DatabaseSummary struct {
	TotalTables        int     `json:"total_tables"`
	TotalRecords       int64   `json:"total_records"`
	AvgRecordsPerTable float64 `json:"avg_records_per_table"`
	LargestTable       string  `json:"largest_table"`
	SmallestTable      string  `json:"smallest_table"`
}

// AnalyzeDatabase 分析整個資料庫
func (a *DatabaseAnalyzer) AnalyzeDatabase(dbSchema *schema.DatabaseSchema) (*AnalysisResult, error) {
	result := &AnalysisResult{
		DatabaseName:  dbSchema.DatabaseInfo.Name,
		AnalysisTime:  time.Now(),
		TableAnalyses: make([]TableAnalysis, 0, len(dbSchema.Tables)),
	}

	var totalRecords int64

	// 分析每個表格
	for _, table := range dbSchema.Tables {
		tableAnalysis, err := a.analyzeTable(table)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze table %s: %w", table.Name, err)
		}

		result.TableAnalyses = append(result.TableAnalyses, *tableAnalysis)
		totalRecords += tableAnalysis.RecordCount
	}

	// 生成摘要
	result.Summary = a.generateDatabaseSummary(result.TableAnalyses, totalRecords)

	return result, nil
}

// analyzeTable 分析單個表格
func (a *DatabaseAnalyzer) analyzeTable(table schema.Table) (*TableAnalysis, error) {
	analysis := &TableAnalysis{
		TableName:      table.Name,
		ColumnAnalyses: make([]ColumnAnalysis, 0, len(table.Columns)),
	}

	// 獲取記錄數
	recordCount, err := a.getRecordCount(table.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get record count: %w", err)
	}
	analysis.RecordCount = recordCount

	// 分析每個欄位
	for _, column := range table.Columns {
		columnAnalysis, err := a.analyzeColumn(table.Name, column, recordCount)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze column %s: %w", column.Name, err)
		}
		analysis.ColumnAnalyses = append(analysis.ColumnAnalyses, *columnAnalysis)
	}

	// 生成表格摘要
	analysis.Summary = a.generateTableSummary(table, analysis.ColumnAnalyses, recordCount)

	return analysis, nil
}

// getRecordCount 獲取表格記錄數
func (a *DatabaseAnalyzer) getRecordCount(tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", a.quoteIdentifier(tableName))

	var count int64
	err := a.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetRecordCount 獲取表格記錄數 (公開方法)
func (a *DatabaseAnalyzer) GetRecordCount(tableName string) (int64, error) {
	return a.getRecordCount(tableName)
}

// analyzeColumn 分析單個欄位
func (a *DatabaseAnalyzer) analyzeColumn(tableName string, column schema.Column, totalRecords int64) (*ColumnAnalysis, error) {
	analysis := &ColumnAnalysis{
		ColumnName: column.Name,
		ColumnType: column.Type,
		TotalCount: totalRecords,
	}

	// 計算非空值數量
	notNullCount, err := a.getNotNullCount(tableName, column.Name)
	if err != nil {
		return nil, err
	}
	analysis.NotNullCount = notNullCount
	analysis.NullCount = totalRecords - notNullCount

	// 計算空值比例，避免除以零
	if totalRecords > 0 {
		analysis.NullRatio = float64(analysis.NullCount) / float64(totalRecords)
	} else {
		analysis.NullRatio = 0.0
	}

	// 計算唯一值數量
	uniqueCount, err := a.getUniqueCount(tableName, column.Name)
	if err != nil {
		return nil, err
	}
	analysis.UniqueCount = uniqueCount

	// 獲取樣本值
	sampleValues, err := a.getSampleValues(tableName, column.Name)
	if err != nil {
		return nil, err
	}
	analysis.SampleValues = sampleValues

	// 對於數值型欄位，計算統計值
	if a.isNumericType(column.Type) && notNullCount > 0 {
		minVal, maxVal, avgVal, err := a.getNumericStats(tableName, column.Name)
		if err == nil {
			analysis.MinValue = minVal
			analysis.MaxValue = maxVal
			analysis.AvgValue = avgVal
		}
	}

	return analysis, nil
}

// AnalyzeColumn 分析單個欄位 (公開方法)
func (a *DatabaseAnalyzer) AnalyzeColumn(tableName string, column schema.Column, totalRecords int64) (*ColumnAnalysis, error) {
	return a.analyzeColumn(tableName, column, totalRecords)
}

// getNotNullCount 獲取非空值數量
func (a *DatabaseAnalyzer) getNotNullCount(tableName, columnName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IS NOT NULL",
		a.quoteIdentifier(tableName), a.quoteIdentifier(columnName))

	var count int64
	err := a.db.QueryRow(query).Scan(&count)
	return count, err
}

// getUniqueCount 獲取唯一值數量
func (a *DatabaseAnalyzer) getUniqueCount(tableName, columnName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s",
		a.quoteIdentifier(columnName), a.quoteIdentifier(tableName))

	var count int64
	err := a.db.QueryRow(query).Scan(&count)
	return count, err
}

// getSampleValues 獲取樣本值
func (a *DatabaseAnalyzer) getSampleValues(tableName, columnName string) ([]string, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NOT NULL LIMIT 5",
		a.quoteIdentifier(columnName), a.quoteIdentifier(tableName), a.quoteIdentifier(columnName))

	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []string
	for rows.Next() {
		var value interface{}
		err := rows.Scan(&value)
		if err != nil {
			continue
		}

		if value != nil {
			samples = append(samples, fmt.Sprintf("%v", value))
		}
	}

	return samples, nil
}

// getNumericStats 獲取數值統計
func (a *DatabaseAnalyzer) getNumericStats(tableName, columnName string) (min, max, avg interface{}, err error) {
	query := fmt.Sprintf("SELECT MIN(%s), MAX(%s), AVG(%s) FROM %s WHERE %s IS NOT NULL",
		a.quoteIdentifier(columnName), a.quoteIdentifier(columnName), a.quoteIdentifier(columnName),
		a.quoteIdentifier(tableName), a.quoteIdentifier(columnName))

	err = a.db.QueryRow(query).Scan(&min, &max, &avg)
	return
}

// generateTableSummary 生成表格摘要
func (a *DatabaseAnalyzer) generateTableSummary(table schema.Table, columnAnalyses []ColumnAnalysis, recordCount int64) TableSummary {
	summary := TableSummary{
		TotalColumns: len(columnAnalyses),
	}

	// 計算主鍵和外鍵數量
	for _, col := range table.Columns {
		if col.IsPrimaryKey {
			summary.PrimaryKeys++
		}
		if col.IsForeignKey {
			summary.ForeignKeys++
		}
	}

	// 計算可空欄位比例
	nullableCount := 0
	totalCompleteness := 0.0

	for _, analysis := range columnAnalyses {
		if analysis.NullCount > 0 {
			nullableCount++
		}
		if recordCount > 0 {
			totalCompleteness += float64(analysis.NotNullCount) / float64(recordCount)
		}
	}

	if summary.TotalColumns > 0 {
		summary.NullableRatio = float64(nullableCount) / float64(summary.TotalColumns)
		summary.DataCompleteness = totalCompleteness / float64(summary.TotalColumns)
	}

	return summary
}

// GenerateTableSummary 生成表格摘要 (公開方法)
func (a *DatabaseAnalyzer) GenerateTableSummary(table schema.Table, columnAnalyses []ColumnAnalysis, recordCount int64) TableSummary {
	return a.generateTableSummary(table, columnAnalyses, recordCount)
}

// generateDatabaseSummary 生成資料庫摘要
func (a *DatabaseAnalyzer) generateDatabaseSummary(tableAnalyses []TableAnalysis, totalRecords int64) DatabaseSummary {
	summary := DatabaseSummary{
		TotalTables:  len(tableAnalyses),
		TotalRecords: totalRecords,
	}

	if summary.TotalTables > 0 {
		summary.AvgRecordsPerTable = float64(totalRecords) / float64(summary.TotalTables)
	}

	// 找出最大和最小的表格
	if len(tableAnalyses) > 0 {
		maxRecords := tableAnalyses[0].RecordCount
		minRecords := tableAnalyses[0].RecordCount
		maxTable := tableAnalyses[0].TableName
		minTable := tableAnalyses[0].TableName

		for _, analysis := range tableAnalyses {
			if analysis.RecordCount > maxRecords {
				maxRecords = analysis.RecordCount
				maxTable = analysis.TableName
			}
			if analysis.RecordCount < minRecords {
				minRecords = analysis.RecordCount
				minTable = analysis.TableName
			}
		}

		summary.LargestTable = maxTable
		summary.SmallestTable = minTable
	}

	return summary
}

// isNumericType 檢查是否為數值型別
func (a *DatabaseAnalyzer) isNumericType(columnType string) bool {
	columnType = strings.ToLower(columnType)
	return strings.Contains(columnType, "int") ||
		strings.Contains(columnType, "decimal") ||
		strings.Contains(columnType, "numeric") ||
		strings.Contains(columnType, "float") ||
		strings.Contains(columnType, "double")
}

// quoteIdentifier 根據資料庫類型引用識別符
func (a *DatabaseAnalyzer) quoteIdentifier(identifier string) string {
	switch strings.ToLower(a.dbType) {
	case "mysql":
		return fmt.Sprintf("`%s`", identifier)
	case "postgres":
		return fmt.Sprintf(`"%s"`, identifier)
	default:
		return identifier
	}
}
