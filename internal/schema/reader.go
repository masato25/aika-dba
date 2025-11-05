package schema

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DatabaseSchema 代表完整的資料庫結構
type DatabaseSchema struct {
	DatabaseInfo  DatabaseInfo   `json:"database_info"`
	Tables        []Table        `json:"tables"`
	Relationships []Relationship `json:"relationships"`
	Metadata      Metadata       `json:"metadata"`
}

// DatabaseInfo 資料庫基本資訊
type DatabaseInfo struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Name    string `json:"name"`
}

// Table 表格結構
type Table struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	EstimatedRows int64    `json:"estimated_rows"`
	Columns       []Column `json:"columns"`
	Indexes       []Index  `json:"indexes"`
}

// Column 欄位結構
type Column struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Nullable     bool     `json:"nullable"`
	DefaultValue *string  `json:"default_value"`
	IsPrimaryKey bool     `json:"is_primary_key"`
	IsForeignKey bool     `json:"is_foreign_key"`
	References   *string  `json:"references"`
	SampleValues []string `json:"sample_values"`
	Description  *string  `json:"description"`
}

// Index 索引結構
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Type    string   `json:"type"`
	Unique  bool     `json:"unique"`
}

// Relationship 表格間關係
type Relationship struct {
	Name             string `json:"name"`
	FromTable        string `json:"from_table"`
	FromColumn       string `json:"from_column"`
	ToTable          string `json:"to_table"`
	ToColumn         string `json:"to_column"`
	RelationshipType string `json:"relationship_type"`
	OnDelete         string `json:"on_delete"`
	OnUpdate         string `json:"on_update"`
}

// Metadata 收集元資料
type Metadata struct {
	CollectedAt          time.Time `json:"collected_at"`
	CollectionDurationMs int64     `json:"collection_duration_ms"`
	TotalTables          int       `json:"total_tables"`
	TotalColumns         int       `json:"total_columns"`
}

// DatabaseConfig 資料庫連接設定
type DatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// SchemaReader 負責讀取資料庫結構
type SchemaReader struct {
	db     *sql.DB
	dbType string
}

// NewSchemaReader 建立新的 SchemaReader
func NewSchemaReader(db *sql.DB, dbType string) *SchemaReader {
	return &SchemaReader{
		db:     db,
		dbType: dbType,
	}
}

// ReadSchema 讀取完整的資料庫結構
func (r *SchemaReader) ReadSchema() (*DatabaseSchema, error) {
	startTime := time.Now()

	// 獲取資料庫基本資訊
	dbInfo, err := r.getDatabaseInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	// 獲取所有表格
	tables, err := r.getTables()
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	// 獲取所有欄位
	totalColumns := 0
	for i := range tables {
		columns, err := r.getColumns(tables[i].Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tables[i].Name, err)
		}
		tables[i].Columns = columns
		totalColumns += len(columns)

		// 獲取索引
		indexes, err := r.getIndexes(tables[i].Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for table %s: %w", tables[i].Name, err)
		}
		tables[i].Indexes = indexes
	}

	// 獲取表格間關係
	relationships, err := r.getRelationships()
	if err != nil {
		return nil, fmt.Errorf("failed to get relationships: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()

	schema := &DatabaseSchema{
		DatabaseInfo:  *dbInfo,
		Tables:        tables,
		Relationships: relationships,
		Metadata: Metadata{
			CollectedAt:          startTime,
			CollectionDurationMs: duration,
			TotalTables:          len(tables),
			TotalColumns:         totalColumns,
		},
	}

	return schema, nil
}

// ToJSON 將 schema 轉換為 JSON 字串
func (s *DatabaseSchema) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveToFile 將 schema 儲存到檔案
func (s *DatabaseSchema) SaveToFile(filename string) error {
	jsonData, err := s.ToJSON()
	if err != nil {
		return err
	}

	file, err := createFile(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(jsonData)
	return err
}

// createFile 建立檔案
func createFile(filename string) (*os.File, error) {
	return os.Create(filename)
}

// getDatabaseInfo 獲取資料庫基本資訊
func (r *SchemaReader) getDatabaseInfo() (*DatabaseInfo, error) {
	var version, dbName string

	switch r.dbType {
	case "postgres":
		// PostgreSQL
		err := r.db.QueryRow("SELECT version(), current_database()").Scan(&version, &dbName)
		if err != nil {
			return nil, err
		}
		// 簡化版本號
		if len(version) > 50 {
			version = version[:50] + "..."
		}
	case "mysql":
		// MySQL
		err := r.db.QueryRow("SELECT VERSION(), DATABASE()").Scan(&version, &dbName)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	return &DatabaseInfo{
		Type:    r.dbType,
		Version: version,
		Name:    dbName,
	}, nil
}

// getTables 獲取所有表格
func (r *SchemaReader) getTables() ([]Table, error) {
	var query string
	var args []interface{}

	switch r.dbType {
	case "postgres":
		query = `
			SELECT
				tablename as table_name,
				'BASE TABLE' as table_type,
				0 as estimated_rows
			FROM pg_tables
			WHERE schemaname = 'public'
			UNION ALL
			SELECT
				viewname as table_name,
				'VIEW' as table_type,
				0 as estimated_rows
			FROM pg_views
			WHERE schemaname = 'public'
		`
	case "mysql":
		query = `
			SELECT
				table_name,
				table_type,
				table_rows as estimated_rows
			FROM information_schema.tables
			WHERE table_schema = DATABASE()
		`
	default:
		return nil, fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var table Table
		err := rows.Scan(&table.Name, &table.Type, &table.EstimatedRows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}

// getColumns 獲取指定表格的欄位
func (r *SchemaReader) getColumns(tableName string) ([]Column, error) {
	var query string
	var args []interface{}

	switch r.dbType {
	case "postgres":
		query = `
			SELECT
				column_name,
				data_type || COALESCE('(' || character_maximum_length::text || ')', '') as column_type,
				is_nullable = 'YES' as nullable,
				column_default,
				is_identity = 'YES' as is_primary_key,
				false as is_foreign_key,
				null as references
			FROM information_schema.columns
			WHERE table_name = $1 AND table_schema = 'public'
			ORDER BY ordinal_position
		`
		args = []interface{}{tableName}
	case "mysql":
		query = `
			SELECT
				column_name,
				column_type,
				is_nullable = 'YES' as nullable,
				column_default,
				column_key = 'PRI' as is_primary_key,
				column_key = 'MUL' as is_foreign_key,
				null as references
			FROM information_schema.columns
			WHERE table_name = ? AND table_schema = DATABASE()
			ORDER BY ordinal_position
		`
		args = []interface{}{tableName}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var defaultValue sql.NullString
		var references sql.NullString

		err := rows.Scan(
			&col.Name,
			&col.Type,
			&col.Nullable,
			&defaultValue,
			&col.IsPrimaryKey,
			&col.IsForeignKey,
			&references,
		)
		if err != nil {
			return nil, err
		}

		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		if references.Valid {
			col.References = &references.String
		}

		// 獲取樣本值
		samples, err := r.getSampleValues(tableName, col.Name)
		if err != nil {
			// 不因為樣本值錯誤而失敗
			samples = []string{}
		}
		col.SampleValues = samples

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// getIndexes 獲取指定表格的索引
func (r *SchemaReader) getIndexes(tableName string) ([]Index, error) {
	var query string
	var args []interface{}

	switch r.dbType {
	case "postgres":
		query = `
			SELECT
				indexname as index_name,
				array_agg(attname ORDER BY attnum) as columns,
				'indices' as index_type,
				indisunique as is_unique
			FROM pg_indexes i
			JOIN pg_class c ON c.relname = i.indexname
			JOIN pg_index idx ON idx.indexrelid = c.oid
			JOIN pg_attribute a ON a.attrelid = idx.indrelid AND a.attnum = ANY(idx.indkey)
			WHERE i.tablename = $1 AND i.schemaname = 'public'
			GROUP BY indexname, indisunique
		`
		args = []interface{}{tableName}
	case "mysql":
		query = `
			SELECT
				index_name,
				GROUP_CONCAT(column_name ORDER BY seq_in_index) as columns,
				index_type,
				non_unique = 0 as is_unique
			FROM information_schema.statistics
			WHERE table_name = ? AND table_schema = DATABASE()
			GROUP BY index_name, index_type, non_unique
		`
		args = []interface{}{tableName}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		var columnsStr string

		err := rows.Scan(&idx.Name, &columnsStr, &idx.Type, &idx.Unique)
		if err != nil {
			return nil, err
		}

		// 解析欄位列表
		idx.Columns = parseColumnsString(columnsStr, r.dbType)
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

// getRelationships 獲取表格間關係
func (r *SchemaReader) getRelationships() ([]Relationship, error) {
	var query string
	var args []interface{}

	switch r.dbType {
	case "postgres":
		query = `
			SELECT
				con.conname as constraint_name,
				child.relname as from_table,
				att2.attname as from_column,
				parent.relname as to_table,
				att.attname as to_column,
				'many_to_one' as relationship_type,
				'NO ACTION' as on_delete,
				'NO ACTION' as on_update
			FROM pg_constraint con
			JOIN pg_class parent ON parent.oid = con.confrelid
			JOIN pg_class child ON child.oid = con.conrelid
			JOIN pg_attribute att ON att.attrelid = parent.oid AND att.attnum = ANY(con.confkey)
			JOIN pg_attribute att2 ON att2.attrelid = child.oid AND att2.attnum = ANY(con.conkey)
			WHERE con.contype = 'f'
		`
	case "mysql":
		query = `
			SELECT
				constraint_name,
				table_name as from_table,
				column_name as from_column,
				referenced_table_name as to_table,
				referenced_column_name as to_column,
				'many_to_one' as relationship_type,
				delete_rule as on_delete,
				update_rule as on_update
			FROM information_schema.key_column_usage kcu
			JOIN information_schema.referential_constraints rc ON kcu.constraint_name = rc.constraint_name
			WHERE kcu.table_schema = DATABASE() AND referenced_table_name IS NOT NULL
		`
	default:
		return nil, fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []Relationship
	for rows.Next() {
		var rel Relationship
		err := rows.Scan(
			&rel.Name,
			&rel.FromTable,
			&rel.FromColumn,
			&rel.ToTable,
			&rel.ToColumn,
			&rel.RelationshipType,
			&rel.OnDelete,
			&rel.OnUpdate,
		)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}

	return relationships, rows.Err()
}

// getSampleValues 獲取欄位樣本值，優先取最新資料
func (r *SchemaReader) getSampleValues(tableName, columnName string) ([]string, error) {
	// 首先嘗試找到時間戳欄位來排序
	sortColumn, err := r.findSortColumn(tableName)
	if err == nil && sortColumn != "" {
		// 使用排序欄位降序排列，獲取最新資料
		query := fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NOT NULL ORDER BY %s DESC LIMIT 5",
			columnName, tableName, columnName, sortColumn)
		return r.executeSampleQuery(query)
	}

	// 如果沒有排序欄位，使用原來的隨機取樣方法
	query := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s IS NOT NULL LIMIT 5", columnName, tableName, columnName)
	return r.executeSampleQuery(query)
}

// findSortColumn 尋找用於排序的欄位（時間欄位或主鍵）
func (r *SchemaReader) findSortColumn(tableName string) (string, error) {
	// 首先嘗試找到時間戳欄位
	timeColumn, err := r.findTimeColumn(tableName)
	if err == nil && timeColumn != "" {
		return timeColumn, nil
	}

	// 如果沒有時間欄位，嘗試找到主鍵欄位
	primaryKey, err := r.findPrimaryKeyColumn(tableName)
	if err == nil && primaryKey != "" {
		return primaryKey, nil
	}

	return "", fmt.Errorf("no suitable sort column found")
}

// findTimeColumn 尋找表格中的時間戳欄位
func (r *SchemaReader) findTimeColumn(tableName string) (string, error) {
	// 常見的時間欄位名稱
	timeColumnNames := []string{
		"created_at", "updated_at", "timestamp", "created_date", "updated_date",
		"create_time", "update_time", "date_created", "date_updated",
		"inserted_at", "modified_at",
	}

	for _, colName := range timeColumnNames {
		// 檢查欄位是否存在且為時間型態
		var exists bool
		var dataType string

		switch r.dbType {
		case "postgres":
			query := `
				SELECT EXISTS(
					SELECT 1 FROM information_schema.columns
					WHERE table_name = $1 AND column_name = $2 AND table_schema = 'public'
				),
				COALESCE((
					SELECT data_type FROM information_schema.columns
					WHERE table_name = $1 AND column_name = $2 AND table_schema = 'public'
				), '')`
			err := r.db.QueryRow(query, tableName, colName).Scan(&exists, &dataType)
			if err != nil {
				continue
			}
		case "mysql":
			query := `
				SELECT EXISTS(
					SELECT 1 FROM information_schema.columns
					WHERE table_name = ? AND column_name = ? AND table_schema = DATABASE()
				),
				COALESCE((
					SELECT data_type FROM information_schema.columns
					WHERE table_name = ? AND column_name = ? AND table_schema = DATABASE()
				), '')`
			err := r.db.QueryRow(query, tableName, colName, tableName, colName).Scan(&exists, &dataType)
			if err != nil {
				continue
			}
		default:
			return "", fmt.Errorf("unsupported database type: %s", r.dbType)
		}

		// 檢查是否為時間型態
		if exists && r.isTimeType(dataType) {
			return colName, nil
		}
	}

	return "", fmt.Errorf("no time column found")
}

// findPrimaryKeyColumn 尋找表格的主鍵欄位
func (r *SchemaReader) findPrimaryKeyColumn(tableName string) (string, error) {
	var query string
	var args []interface{}

	switch r.dbType {
	case "postgres":
		query = `
			SELECT a.attname as column_name
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = $1::regclass AND i.indisprimary
			LIMIT 1
		`
		args = []interface{}{tableName}
	case "mysql":
		query = `
			SELECT column_name
			FROM information_schema.key_column_usage
			WHERE table_name = ? AND table_schema = DATABASE() AND constraint_name = 'PRIMARY'
			LIMIT 1
		`
		args = []interface{}{tableName}
	default:
		return "", fmt.Errorf("unsupported database type: %s", r.dbType)
	}

	var primaryKey string
	err := r.db.QueryRow(query, args...).Scan(&primaryKey)
	if err != nil {
		return "", err
	}

	return primaryKey, nil
}

// isTimeType 檢查是否為時間型態
func (r *SchemaReader) isTimeType(dataType string) bool {
	timeTypes := []string{
		"timestamp", "datetime", "date", "time",
		"timestamp without time zone", "timestamp with time zone",
	}

	dataType = strings.ToLower(dataType)
	for _, t := range timeTypes {
		if strings.Contains(dataType, t) {
			return true
		}
	}
	return false
}

// executeSampleQuery 執行樣本查詢
func (r *SchemaReader) executeSampleQuery(query string) ([]string, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []string
	for rows.Next() {
		var value sql.NullString
		err := rows.Scan(&value)
		if err != nil {
			continue
		}
		if value.Valid && len(samples) < 5 {
			// 限制樣本值長度
			sample := value.String
			if len(sample) > 100 {
				sample = sample[:100] + "..."
			}
			samples = append(samples, sample)
		}
	}

	return samples, rows.Err()
}

// parseColumnsString 解析欄位字串
func parseColumnsString(columnsStr string, dbType string) []string {
	if dbType == "postgres" {
		// PostgreSQL array format: {col1,col2,col3}
		if len(columnsStr) > 2 && columnsStr[0] == '{' && columnsStr[len(columnsStr)-1] == '}' {
			columnsStr = columnsStr[1 : len(columnsStr)-1]
		}
	}
	// 簡單的逗號分割（實際實現可能需要更複雜的解析）
	return strings.Split(columnsStr, ",")
}

// ConnectDatabase 連接到資料庫
func ConnectDatabase(config DatabaseConfig) (*sql.DB, error) {
	var dsn string

	switch config.Type {
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.User, config.Password, config.DBName)
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			config.User, config.Password, config.Host, config.Port, config.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	db, err := sql.Open(config.Type, dsn)
	if err != nil {
		return nil, err
	}

	// 測試連接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
