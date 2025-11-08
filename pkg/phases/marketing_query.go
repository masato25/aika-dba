package phases

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// MarketingQueryRunner 營銷查詢執行器 - 結合向量搜索和 SQL 執行來回答自然語言業務問題
type MarketingQueryRunner struct {
	config       *config.Config
	db           *sql.DB
	knowledgeMgr *vectorstore.KnowledgeManager
	llmClient    *llm.Client
}

// NewMarketingQueryRunner 創建營銷查詢執行器
func NewMarketingQueryRunner(cfg *config.Config, db *sql.DB) *MarketingQueryRunner {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Printf("Warning: Failed to create knowledge manager: %v", err)
		knowledgeMgr = nil
	}

	return &MarketingQueryRunner{
		config:       cfg,
		db:           db,
		knowledgeMgr: knowledgeMgr,
		llmClient:    llm.NewClient(cfg),
	}
}

// QueryResult 查詢結果結構
type QueryResult struct {
	Query       string                   `json:"query"`
	SQLQuery    string                   `json:"sql_query,omitempty"`
	Results     []map[string]interface{} `json:"results,omitempty"`
	Explanation string                   `json:"explanation"`
	Timestamp   time.Time                `json:"timestamp"`
	Error       string                   `json:"error,omitempty"`
}

// ExecuteMarketingQuery 執行營銷查詢
func (m *MarketingQueryRunner) ExecuteMarketingQuery(naturalLanguageQuery string) (*QueryResult, error) {
	log.Printf("=== Executing Marketing Query: %s ===", naturalLanguageQuery)

	result := &QueryResult{
		Query:     naturalLanguageQuery,
		Timestamp: time.Now(),
	}

	// 步驟 1: 從向量存儲檢索相關業務知識
	relevantKnowledge, err := m.retrieveRelevantKnowledge(naturalLanguageQuery)
	if err != nil {
		log.Printf("Warning: Failed to retrieve relevant knowledge: %v", err)
		relevantKnowledge = "No relevant business knowledge found."
	}

	// 步驟 2: 生成 SQL 查詢
	sqlQuery, explanation, err := m.generateSQLQuery(naturalLanguageQuery, relevantKnowledge)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to generate SQL query: %v", err)
		return result, nil
	}

	result.SQLQuery = sqlQuery
	result.Explanation = explanation

	// 步驟 3: 執行 SQL 查詢
	queryResults, err := m.executeSQLQuery(sqlQuery)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to execute SQL query: %v", err)
		return result, nil
	}

	result.Results = queryResults

	log.Printf("Marketing query executed successfully, returned %d results", len(queryResults))
	return result, nil
}

// retrieveRelevantKnowledge 從向量存儲檢索相關業務知識
func (m *MarketingQueryRunner) retrieveRelevantKnowledge(query string) (string, error) {
	if m.knowledgeMgr == nil {
		return "", fmt.Errorf("knowledge manager not available")
	}

	// 從所有 phase 檢索相關知識
	var allKnowledge []string

	// 對於生日相關查詢，優先檢索 customers 表的信息
	if strings.Contains(strings.ToLower(query), "生日") || strings.Contains(strings.ToLower(query), "birth") {
		// 專門檢索 customers 表的 schema 信息
		customerResults, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase1", "customers table schema date_of_birth", 3)
		if err == nil {
			for _, result := range customerResults {
				allKnowledge = append(allKnowledge, fmt.Sprintf("CUSTOMERS TABLE SCHEMA: %s", result.Content))
			}
		}
	}

	// 檢索 Phase 1 知識 (架構分析) - 增加檢索數量
	phase1Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, 3)
	if err == nil {
		for _, result := range phase1Results {
			// 對於生日查詢，優先保留包含 customers 或 date_of_birth 的內容
			content := result.Content
			if (strings.Contains(strings.ToLower(query), "生日") || strings.Contains(strings.ToLower(query), "birth")) &&
				(strings.Contains(strings.ToLower(content), "customers") || strings.Contains(strings.ToLower(content), "date_of_birth")) {
				allKnowledge = append(allKnowledge, fmt.Sprintf("PHASE 1 - SCHEMA (HIGH PRIORITY): %s", content))
			} else {
				// 截斷其他內容以適應 LLM 上下文限制
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 1 - Schema Analysis: %s", content))
			}
		}
	}

	// 檢索 Phase 2 知識 (業務邏輯分析)
	phase2Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, 2)
	if err == nil {
		for _, result := range phase2Results {
			// 截斷知識內容以適應 LLM 上下文限制
			truncatedContent := result.Content
			if len(truncatedContent) > 400 {
				truncatedContent = truncatedContent[:400] + "..."
			}
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 2 - Business Logic: %s", truncatedContent))
		}
	}

	// 檢索 Phase 3 知識 (商業邏輯描述)
	phase3Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase3", query, 2)
	if err == nil {
		for _, result := range phase3Results {
			// 截斷知識內容以適應 LLM 上下文限制
			truncatedContent := result.Content
			if len(truncatedContent) > 400 {
				truncatedContent = truncatedContent[:400] + "..."
			}
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 3 - Business Description: %s", truncatedContent))
		}
	}

	if len(allKnowledge) == 0 {
		return "No relevant business knowledge found in vector store.", nil
	}

	return strings.Join(allKnowledge, "\n\n"), nil
}

// generateSQLQuery 生成 SQL 查詢
func (m *MarketingQueryRunner) generateSQLQuery(naturalLanguageQuery, relevantKnowledge string) (string, string, error) {
	// 獲取數據庫架構信息
	schemaInfo, err := m.getDatabaseSchemaInfo()
	if err != nil {
		return "", "", fmt.Errorf("failed to get database schema: %v", err)
	}

	// 調試：打印 schema 信息
	log.Printf("DEBUG - Schema Info length: %d", len(schemaInfo))
	log.Printf("DEBUG - Schema Info preview: %s", schemaInfo[:min(500, len(schemaInfo))])
	log.Printf("DEBUG - Relevant Knowledge length: %d", len(relevantKnowledge))
	log.Printf("DEBUG - Relevant Knowledge preview: %s", relevantKnowledge[:min(500, len(relevantKnowledge))])

	// 構造簡單的 LLM 提示
	prompt := fmt.Sprintf(`You are an expert SQL analyst for an e-commerce database. Generate a SQL query to answer the user's question.

DATABASE SCHEMA:
%s

BUSINESS KNOWLEDGE:
%s

USER QUERY: %s

IMPORTANT: The customers table has a 'date_of_birth' field (date, NULL) that stores customer birthdays. Use EXTRACT(MONTH FROM date_of_birth) to get the birth month.

Return ONLY the SQL SELECT statement, no explanations or markdown.`, schemaInfo, relevantKnowledge, naturalLanguageQuery)

	// 調用 LLM 生成 SQL
	response, err := m.llmClient.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		return "", "", fmt.Errorf("LLM failed to generate SQL query: %v", err)
	}

	// 清理響應，提取 SQL 查詢
	sqlQuery := strings.TrimSpace(response)

	// 移除可能的 markdown 代碼塊標記
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimPrefix(sqlQuery, "```")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")

	// 如果響應包含多行，只取第一個 SELECT 語句
	lines := strings.Split(sqlQuery, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "SELECT") {
			// 找到 SELECT 語句，從這裡開始取，直到分號或結束
			sqlQuery = line
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				sqlQuery += " " + nextLine
				if strings.Contains(nextLine, ";") {
					break
				}
			}
			break
		}
	}

	sqlQuery = strings.TrimSpace(sqlQuery)
	// 移除末尾的分號
	sqlQuery = strings.TrimSuffix(sqlQuery, ";")

	log.Printf("Generated SQL query: %s", sqlQuery)

	// 驗證 SQL 查詢安全性
	if !m.isSafeSQLQuery(sqlQuery) {
		log.Printf("SQL query failed security validation: %s", sqlQuery)
		return "", "", fmt.Errorf("generated SQL query failed security validation")
	}

	// 確保是 SELECT 查詢
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sqlQuery)), "SELECT") {
		return "", "", fmt.Errorf("generated query is not a SELECT statement")
	}

	explanation := fmt.Sprintf("SQL query generated by LLM to answer: %s", naturalLanguageQuery)

	return sqlQuery, explanation, nil
}

// getDatabaseSchemaInfo 獲取數據庫架構信息
func (m *MarketingQueryRunner) getDatabaseSchemaInfo() (string, error) {
	// 查詢所有表格及其詳細欄位信息
	rows, err := m.db.Query(`
		SELECT
			t.table_name,
			c.column_name,
			c.data_type,
			CASE WHEN c.is_nullable = 'NO' THEN 'NOT NULL' ELSE 'NULL' END as nullable,
			c.column_default
		FROM information_schema.tables t
		JOIN information_schema.columns c ON t.table_name = c.table_name AND t.table_schema = c.table_schema
		WHERE t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_name, c.ordinal_position
	`)
	if err != nil {
		return "", fmt.Errorf("failed to query schema: %v", err)
	}
	defer rows.Close()

	var schemaInfo strings.Builder
	schemaInfo.WriteString("Database Tables and Columns:\n\n")

	currentTable := ""
	for rows.Next() {
		var tableName, columnName, dataType, nullable, columnDefault string
		if err := rows.Scan(&tableName, &columnName, &dataType, &nullable, &columnDefault); err != nil {
			continue
		}

		if tableName != currentTable {
			if currentTable != "" {
				schemaInfo.WriteString("\n")
			}
			schemaInfo.WriteString(fmt.Sprintf("Table: %s\n", tableName))
			currentTable = tableName
		}

		defaultStr := ""
		if columnDefault != "" && columnDefault != "NULL" {
			defaultStr = fmt.Sprintf(" DEFAULT %s", columnDefault)
		}

		schemaInfo.WriteString(fmt.Sprintf("  - %s (%s, %s%s)\n", columnName, dataType, nullable, defaultStr))
	}

	// 添加一些常見的業務邏輯提示
	schemaInfo.WriteString("\nBusiness Logic Notes:\n")
	schemaInfo.WriteString("- customers 表包含會員信息，可能有 date_of_birth 字段用於生日分析\n")
	schemaInfo.WriteString("- orders 表包含訂單信息，可以聯接到 customers 表進行會員分析\n")
	schemaInfo.WriteString("- products 表包含產品信息\n")
	schemaInfo.WriteString("- reviews 表包含評價信息\n")

	return schemaInfo.String(), nil
}

// isSafeSQLQuery 檢查 SQL 查詢是否安全
func (m *MarketingQueryRunner) isSafeSQLQuery(query string) bool {
	upperQuery := strings.ToUpper(strings.TrimSpace(query))

	// 只允許 SELECT 查詢
	if !strings.HasPrefix(upperQuery, "SELECT") {
		log.Printf("Query rejected: not a SELECT statement")
		return false
	}

	// 不允許危險的關鍵字 (使用詞邊界檢查)
	dangerousKeywords := []string{
		"DROP", "DELETE", "UPDATE", "INSERT", "ALTER", "CREATE", "TRUNCATE",
		"EXEC", "EXECUTE", "MERGE", "BULK", "BACKUP", "RESTORE",
	}

	// 將查詢分割成單詞來檢查
	words := strings.Fields(upperQuery)
	for _, word := range words {
		// 移除標點符號
		word = strings.Trim(word, ".,;()[]")
		for _, keyword := range dangerousKeywords {
			if word == keyword {
				log.Printf("Query rejected: contains dangerous keyword '%s'", keyword)
				return false
			}
		}
	}

	return true
}

// executeSQLQuery 執行 SQL 查詢
func (m *MarketingQueryRunner) executeSQLQuery(sqlQuery string) ([]map[string]interface{}, error) {
	// 執行查詢
	rows, err := m.db.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	// 獲取欄位名稱
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	// 讀取結果
	var results []map[string]interface{}
	count := 0
	const maxRows = 50

	for rows.Next() && count < maxRows {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val != nil {
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

		results = append(results, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %v", err)
	}

	return results, nil
}

// SaveQueryResult 保存查詢結果
func (m *MarketingQueryRunner) SaveQueryResult(result *QueryResult) error {
	if m.knowledgeMgr == nil {
		return fmt.Errorf("knowledge manager not available")
	}

	// 存儲到向量數據庫
	queryKnowledge := map[string]interface{}{
		"phase":        "marketing_query",
		"description":  "Marketing query result",
		"query":        result.Query,
		"sql_query":    result.SQLQuery,
		"result_count": len(result.Results),
		"has_error":    result.Error != "",
		"timestamp":    result.Timestamp,
	}

	return m.knowledgeMgr.StorePhaseKnowledge("marketing_query", queryKnowledge)
}

// min 返回兩個整數中的較小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
