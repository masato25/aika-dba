package analyzer

import (
	"database/sql"
	"fmt"
	"strings"
)

// DatabaseAnalyzer 資料庫分析器
type DatabaseAnalyzer struct {
	db *sql.DB
}

// NewDatabaseAnalyzer 創建資料庫分析器
func NewDatabaseAnalyzer(db *sql.DB) *DatabaseAnalyzer {
	return &DatabaseAnalyzer{db: db}
}

// GetAllTables 獲取所有表格名稱
func (a *DatabaseAnalyzer) GetAllTables() ([]string, error) {
	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`

	rows, err := a.db.Query(query)
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

	return tables, rows.Err()
}

// GetTableSchema 獲取表格的 schema 信息
func (a *DatabaseAnalyzer) GetTableSchema(tableName string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale
		FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position
	`

	rows, err := a.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schema []map[string]interface{}
	for rows.Next() {
		var colName, dataType string
		var isNullable string
		var columnDefault sql.NullString
		var charMaxLen, numPrecision, numScale sql.NullInt64

		err := rows.Scan(&colName, &dataType, &isNullable, &columnDefault, &charMaxLen, &numPrecision, &numScale)
		if err != nil {
			return nil, err
		}

		column := map[string]interface{}{
			"name":       colName,
			"type":       dataType,
			"nullable":   isNullable == "YES",
			"max_length": charMaxLen.Int64,
			"precision":  numPrecision.Int64,
			"scale":      numScale.Int64,
		}

		if columnDefault.Valid {
			column["default"] = columnDefault.String
		}

		schema = append(schema, column)
	}

	return schema, rows.Err()
}

// GetTableConstraints 獲取表格的約束信息（外鍵、主鍵等）
func (a *DatabaseAnalyzer) GetTableConstraints(tableName string) (map[string]interface{}, error) {
	constraints := map[string]interface{}{
		"primary_keys": []string{},
		"foreign_keys": []map[string]interface{}{},
		"unique_keys":  []map[string]interface{}{},
	}

	// 獲取主鍵
	pkQuery := `
		SELECT kc.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kc ON tc.constraint_name = kc.constraint_name
		WHERE tc.table_name = $1 AND tc.table_schema = 'public' AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kc.ordinal_position
	`

	pkRows, err := a.db.Query(pkQuery, tableName)
	if err == nil {
		defer pkRows.Close()
		var pks []string
		for pkRows.Next() {
			var colName string
			if err := pkRows.Scan(&colName); err == nil {
				pks = append(pks, colName)
			}
		}
		constraints["primary_keys"] = pks
	}

	// 獲取外鍵
	fkQuery := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name
		WHERE tc.table_name = $1 AND tc.table_schema = 'public' AND tc.constraint_type = 'FOREIGN KEY'
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	fkRows, err := a.db.Query(fkQuery, tableName)
	if err == nil {
		defer fkRows.Close()
		var fks []map[string]interface{}
		for fkRows.Next() {
			var constraintName, columnName, refTable, refColumn string
			if err := fkRows.Scan(&constraintName, &columnName, &refTable, &refColumn); err == nil {
				fks = append(fks, map[string]interface{}{
					"constraint_name":   constraintName,
					"column":           columnName,
					"referenced_table": refTable,
					"referenced_column": refColumn,
				})
			}
		}
		constraints["foreign_keys"] = fks
	}

	// 獲取唯一鍵
	ukQuery := `
		SELECT
			tc.constraint_name,
			kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = $1 AND tc.table_schema = 'public' AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	ukRows, err := a.db.Query(ukQuery, tableName)
	if err == nil {
		defer ukRows.Close()
		ukMap := make(map[string][]string)
		for ukRows.Next() {
			var constraintName, columnName string
			if err := ukRows.Scan(&constraintName, &columnName); err == nil {
				ukMap[constraintName] = append(ukMap[constraintName], columnName)
			}
		}

		var uks []map[string]interface{}
		for constraintName, columns := range ukMap {
			uks = append(uks, map[string]interface{}{
				"constraint_name": constraintName,
				"columns":        columns,
			})
		}
		constraints["unique_keys"] = uks
	}

	return constraints, nil
}

// GetTableIndexes 獲取表格的索引信息
func (a *DatabaseAnalyzer) GetTableIndexes(tableName string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			indexname,
			indexdef
		FROM pg_indexes
		WHERE tablename = $1 AND schemaname = 'public'
		ORDER BY indexname
	`

	rows, err := a.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []map[string]interface{}
	for rows.Next() {
		var indexName, indexDef string
		if err := rows.Scan(&indexName, &indexDef); err != nil {
			continue
		}

		// 解析索引定義來獲取更多信息
		index := map[string]interface{}{
			"name":       indexName,
			"definition": indexDef,
		}

		// 簡單的索引類型判斷
		if strings.Contains(indexDef, "UNIQUE") {
			index["is_unique"] = true
		} else {
			index["is_unique"] = false
		}

		// 提取欄位（簡單解析）
		if strings.Contains(indexDef, "(") && strings.Contains(indexDef, ")") {
			start := strings.Index(indexDef, "(")
			end := strings.LastIndex(indexDef, ")")
			if start < end {
				columnsStr := indexDef[start+1 : end]
				// 簡單分割，多欄位索引用逗號分隔
				columns := strings.Split(columnsStr, ",")
				for i, col := range columns {
					columns[i] = strings.TrimSpace(col)
				}
				index["columns"] = columns
			}
		}

		indexes = append(indexes, index)
	}

	return indexes, rows.Err()
}

// GetTableSamples 獲取表格的樣本數據
func (a *DatabaseAnalyzer) GetTableSamples(tableName string, maxSamples int) ([]map[string]interface{}, error) {
	// 檢查表格是否有 created_at 或 updated_at 欄位來排序
	hasTimestamp := false
	schema, err := a.GetTableSchema(tableName)
	if err != nil {
		return nil, err
	}

	for _, col := range schema {
		colName := col["name"].(string)
		if colName == "created_at" || colName == "updated_at" {
			hasTimestamp = true
			break
		}
	}

	// 構建查詢
	var query string
	if hasTimestamp {
		query = fmt.Sprintf("SELECT * FROM %s ORDER BY COALESCE(updated_at, created_at) DESC LIMIT %d", tableName, maxSamples)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, maxSamples)
	}

	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 獲取欄位名稱
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var samples []map[string]interface{}
	for rows.Next() {
		// 動態掃描所有欄位
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val != nil {
				// 處理不同類型的數據
				switch v := val.(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = v
				}
			} else {
				row[col] = nil
			}
		}

		samples = append(samples, row)
	}

	return samples, rows.Err()
}

// GetTableStats 獲取表格的統計信息
func (a *DatabaseAnalyzer) GetTableStats(tableName string) (map[string]interface{}, error) {
	// 獲取總行數
	var rowCount int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := a.db.QueryRow(countQuery).Scan(&rowCount)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"row_count": rowCount,
	}

	// 嘗試獲取表格大小信息（PostgreSQL 特定）
	sizeQuery := `
		SELECT
			pg_size_pretty(pg_total_relation_size($1)) as total_size,
			pg_size_pretty(pg_relation_size($1)) as table_size,
			pg_size_pretty(pg_total_relation_size($1) - pg_relation_size($1)) as index_size
	`

	var totalSize, tableSize, indexSize string
	err = a.db.QueryRow(sizeQuery, tableName).Scan(&totalSize, &tableSize, &indexSize)
	if err == nil {
		stats["total_size"] = totalSize
		stats["table_size"] = tableSize
		stats["index_size"] = indexSize
	}

	return stats, nil
}

// AnalyzeTable 分析單個表格，返回完整的分析結果
func (a *DatabaseAnalyzer) AnalyzeTable(tableName string, maxSamples int) (map[string]interface{}, error) {
	// 獲取表格 schema
	schema, err := a.GetTableSchema(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for table %s: %w", tableName, err)
	}

	// 獲取表格約束
	constraints, err := a.GetTableConstraints(tableName)
	if err != nil {
		constraints = map[string]interface{}{}
	}

	// 獲取表格索引
	indexes, err := a.GetTableIndexes(tableName)
	if err != nil {
		indexes = []map[string]interface{}{}
	}

	// 獲取樣本數據
	samples, err := a.GetTableSamples(tableName, maxSamples)
	if err != nil {
		samples = []map[string]interface{}{}
	}

	// 獲取表格統計
	stats, err := a.GetTableStats(tableName)
	if err != nil {
		stats = map[string]interface{}{}
	}

	return map[string]interface{}{
		"schema":      schema,
		"constraints": constraints,
		"indexes":     indexes,
		"samples":     samples,
		"stats":       stats,
	}, nil
}