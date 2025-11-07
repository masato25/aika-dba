package preparer

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// KnowledgePreparer 負責準備知識庫
type KnowledgePreparer struct {
	db     *sql.DB
	km     *vectorstore.KnowledgeManager
	logger *log.Logger
}

// NewKnowledgePreparer 創建新的知識庫準備器
func NewKnowledgePreparer(db *sql.DB, km *vectorstore.KnowledgeManager, logger *log.Logger) *KnowledgePreparer {
	return &KnowledgePreparer{
		db:     db,
		km:     km,
		logger: logger,
	}
}

// PrepareKnowledge 準備知識庫
func (kp *KnowledgePreparer) PrepareKnowledge() error {
	kp.logger.Println("開始準備知識庫...")

	// 獲取所有表格
	tables, err := kp.getTables()
	if err != nil {
		kp.logger.Printf("獲取表格失敗: %v", err)
		return fmt.Errorf("獲取表格失敗: %v", err)
	}

	kp.logger.Printf("找到 %d 個表格", len(tables))

	knowledge := map[string]interface{}{
		"database": "current_db", // 可從配置獲取
		"tables":   []interface{}{},
	}

	analyzedCount := 0
	ignoredTables := []string{}

	for _, tableName := range tables {
		kp.logger.Printf("處理表格: %s", tableName)

		// 檢查行數
		rowCount, err := kp.getRowCount(tableName)
		if err != nil {
			kp.logger.Printf("警告: 無法獲取 %s 的行數: %v", tableName, err)
			continue
		}

		if rowCount == 0 {
			ignoredTables = append(ignoredTables, tableName)
			continue
		}

		// 獲取欄位資訊
		columns, err := kp.getColumns(tableName)
		if err != nil {
			kp.logger.Printf("警告: 無法獲取 %s 的欄位: %v", tableName, err)
			continue
		}

		// 檢測狀態欄位
		statusColumns := kp.detectStatusColumns(tableName, columns)

		// 構建表格知識
		tableKnowledge := map[string]interface{}{
			"name":           tableName,
			"row_count":      rowCount,
			"columns":        kp.columnsToStrings(columns),
			"status_columns": statusColumns,
		}

		knowledge["tables"] = append(knowledge["tables"].([]interface{}), tableKnowledge)
		analyzedCount++
	}

	// 添加統計
	knowledge["analyzed_count"] = analyzedCount
	knowledge["ignored_count"] = len(ignoredTables)
	knowledge["ignored_tables"] = ignoredTables

	kp.logger.Printf("準備存儲知識: analyzed=%d, ignored=%d", analyzedCount, len(ignoredTables))

	// 存儲知識
	err = kp.km.StorePhaseKnowledge("database_knowledge", knowledge)
	if err != nil {
		kp.logger.Printf("存儲知識失敗: %v", err)
		return fmt.Errorf("存儲知識失敗: %v", err)
	}

	kp.logger.Printf("知識庫準備完成: 分析 %d 個表格, 忽略 %d 個空表格", analyzedCount, len(ignoredTables))
	return nil
}

// getTables 獲取所有表格名稱
func (kp *KnowledgePreparer) getTables() ([]string, error) {
	rows, err := kp.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}
	return tables, nil
}

// getRowCount 獲取表格行數
func (kp *KnowledgePreparer) getRowCount(tableName string) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := kp.db.QueryRow(query).Scan(&count)
	return count, err
}

// ColumnInfo 欄位資訊
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
}

// getColumns 獲取欄位資訊
func (kp *KnowledgePreparer) getColumns(tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position
	`
	rows, err := kp.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable); err != nil {
			return nil, err
		}
		col.Nullable = nullable == "YES"
		columns = append(columns, col)
	}
	return columns, nil
}

// detectStatusColumns 檢測狀態欄位
func (kp *KnowledgePreparer) detectStatusColumns(tableName string, columns []ColumnInfo) map[string][]string {
	statusColumns := make(map[string][]string)
	for _, col := range columns {
		if kp.isStatusColumn(col.Name) {
			values, err := kp.getDistinctValues(tableName, col.Name)
			if err != nil {
				kp.logger.Printf("警告: 無法獲取 %s.%s 的唯一值: %v", tableName, col.Name, err)
				continue
			}
			if len(values) <= 20 { // 假設狀態列表不超過 20 個
				statusColumns[col.Name] = values
			}
		}
	}
	return statusColumns
}

// isStatusColumn 判斷是否為狀態欄位
func (kp *KnowledgePreparer) isStatusColumn(columnName string) bool {
	lowerName := strings.ToLower(columnName)
	return strings.Contains(lowerName, "status") ||
		strings.Contains(lowerName, "state") ||
		strings.Contains(lowerName, "type")
}

// getDistinctValues 獲取欄位的唯一值
func (kp *KnowledgePreparer) getDistinctValues(tableName, columnName string) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT %s FROM %s LIMIT 100", columnName, tableName)
	rows, err := kp.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		values = append(values, val)
	}
	return values, nil
}

// columnsToStrings 將欄位轉為字串列表
func (kp *KnowledgePreparer) columnsToStrings(columns []ColumnInfo) []string {
	var result []string
	for _, col := range columns {
		result = append(result, col.Name)
	}
	return result
}

// quoteIdentifier 正確引用標識符以處理特殊字符
func (kp *KnowledgePreparer) quoteIdentifier(identifier string) string {
	// 對於 PostgreSQL，使用雙引號
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
