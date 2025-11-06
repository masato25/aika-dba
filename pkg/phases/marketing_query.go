package phases

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// MarketingQueryRunner 營銷查詢執行器 - 結合向量搜索和 SQL 執行來回答自然語言業務問題
type MarketingQueryRunner struct {
	config       *config.Config
	db           *sql.DB
	knowledgeMgr *vectorstore.KnowledgeManager
	llmClient    *LLMClient
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
		llmClient:    NewLLMClient(cfg),
	}
}

// QueryResult 查詢結果結構
type QueryResult struct {
	Query            string                   `json:"query"`
	SQLQuery         string                   `json:"sql_query,omitempty"`
	Results          []map[string]interface{} `json:"results,omitempty"`
	Explanation      string                   `json:"explanation"`
	BusinessInsights string                   `json:"business_insights,omitempty"`
	Timestamp        time.Time                `json:"timestamp"`
	Error            string                   `json:"error,omitempty"`
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

	// 步驟 4: 生成業務洞察
	businessInsights, err := m.generateBusinessInsights(naturalLanguageQuery, queryResults, relevantKnowledge)
	if err != nil {
		log.Printf("Warning: Failed to generate business insights: %v", err)
		businessInsights = "Unable to generate business insights at this time."
	}

	result.BusinessInsights = businessInsights

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

	// 檢索 Phase 1 知識 (架構分析)
	phase1Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, 5)
	if err == nil {
		for _, result := range phase1Results {
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 1 - Schema Analysis: %s", result.Content))
		}
	}

	// 檢索 Phase 2 知識 (業務邏輯分析)
	phase2Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, 5)
	if err == nil {
		for _, result := range phase2Results {
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 2 - Business Logic: %s", result.Content))
		}
	}

	// 檢索 Phase 3 知識 (商業邏輯描述)
	phase3Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase3", query, 5)
	if err == nil {
		for _, result := range phase3Results {
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 3 - Business Description: %s", result.Content))
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

	// 構造 LLM 提示
	prompt := fmt.Sprintf(`You are a SQL expert. Based on the following database schema and business knowledge, generate a SQL query to answer the user's question.

Database Schema:
%s

Business Knowledge:
%s

User Question: %s

Instructions:
1. Generate a SELECT query that answers the question
2. Use proper table joins if needed
3. Include appropriate WHERE, GROUP BY, ORDER BY clauses
4. Limit results to 50 rows maximum for performance
5. Return only the SQL query, no explanations

SQL Query:`, schemaInfo, relevantKnowledge, naturalLanguageQuery)

	// 調用 LLM 生成 SQL
	response, err := m.llmClient.CallLLM(prompt)
	if err != nil {
		// 後備模式：使用簡單的規則生成 SQL
		log.Printf("LLM failed, using fallback SQL generation: %v", err)
		sqlQuery, explanation := m.fallbackSQLGeneration(naturalLanguageQuery)
		return sqlQuery, explanation, nil
	}

	// 清理響應，提取 SQL 查詢
	sqlQuery := strings.TrimSpace(response)
	sqlQuery = strings.TrimPrefix(sqlQuery, "SQL Query:")
	sqlQuery = strings.TrimSpace(sqlQuery)

	// 驗證 SQL 查詢安全性
	if !m.isSafeSQLQuery(sqlQuery) {
		return "", "", fmt.Errorf("generated SQL query is not safe")
	}

	explanation := fmt.Sprintf("Generated SQL query based on business knowledge and database schema to answer: %s", naturalLanguageQuery)

	return sqlQuery, explanation, nil
}

// getDatabaseSchemaInfo 獲取數據庫架構信息
func (m *MarketingQueryRunner) getDatabaseSchemaInfo() (string, error) {
	// 查詢所有表格及其欄位
	rows, err := m.db.Query(`
		SELECT
			t.table_name,
			array_agg(c.column_name || ' ' || c.data_type || CASE WHEN c.is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END) as columns
		FROM information_schema.tables t
		JOIN information_schema.columns c ON t.table_name = c.table_name
		WHERE t.table_schema = 'public'
		GROUP BY t.table_name
		ORDER BY t.table_name
	`)
	if err != nil {
		return "", fmt.Errorf("failed to query schema: %v", err)
	}
	defer rows.Close()

	var schemaInfo strings.Builder
	schemaInfo.WriteString("Database Tables:\n")

	for rows.Next() {
		var tableName string
		var columns []string
		if err := rows.Scan(&tableName, &columns); err != nil {
			continue
		}

		schemaInfo.WriteString(fmt.Sprintf("\nTable: %s\n", tableName))
		for _, col := range columns {
			schemaInfo.WriteString(fmt.Sprintf("  - %s\n", col))
		}
	}

	return schemaInfo.String(), nil
}

// isSafeSQLQuery 檢查 SQL 查詢是否安全
func (m *MarketingQueryRunner) isSafeSQLQuery(query string) bool {
	upperQuery := strings.ToUpper(strings.TrimSpace(query))

	// 只允許 SELECT 查詢
	if !strings.HasPrefix(upperQuery, "SELECT") {
		return false
	}

	// 不允許危險的關鍵字
	dangerousKeywords := []string{
		"DROP", "DELETE", "UPDATE", "INSERT", "ALTER", "CREATE", "TRUNCATE",
		"EXEC", "EXECUTE", "MERGE", "BULK", "BACKUP", "RESTORE",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(upperQuery, keyword) {
			return false
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

// generateBusinessInsights 生成業務洞察
func (m *MarketingQueryRunner) generateBusinessInsights(query string, results []map[string]interface{}, knowledge string) (string, error) {
	if len(results) == 0 {
		return "No data available to generate insights.", nil
	}

	// 構造 LLM 提示
	resultSummary := fmt.Sprintf("Query returned %d rows with %d columns", len(results), len(results[0]))

	prompt := fmt.Sprintf(`You are a business analyst. Based on the following query, results, and business knowledge, provide key business insights.

User Query: %s

Query Results Summary: %s

Business Knowledge: %s

Please provide:
1. Key findings from the data
2. Business implications
3. Recommendations for action
4. Any trends or patterns observed

Keep the response concise but insightful.`, query, resultSummary, knowledge)

	// 調用 LLM 生成洞察
	response, err := m.llmClient.CallLLM(prompt)
	if err != nil {
		// 後備模式：生成簡單的洞察
		log.Printf("LLM failed for insights, using fallback: %v", err)
		return m.fallbackBusinessInsights(query, results), nil
	}

	return strings.TrimSpace(response), nil
}

// fallbackBusinessInsights 後備業務洞察生成
func (m *MarketingQueryRunner) fallbackBusinessInsights(query string, results []map[string]interface{}) string {
	queryLower := strings.ToLower(query)

	if strings.Contains(queryLower, "how many customers") && len(results) > 0 {
		if count, ok := results[0]["customer_count"]; ok {
			return fmt.Sprintf("The system has %v registered customers. This represents the total customer base available for marketing campaigns and analysis.", count)
		}
	}

	if strings.Contains(queryLower, "total orders") && len(results) > 0 {
		if count, ok := results[0]["order_count"]; ok {
			return fmt.Sprintf("There are %v total orders in the system. This indicates the overall transaction volume and business activity level.", count)
		}
	}

	if strings.Contains(queryLower, "total revenue") && len(results) > 0 {
		if revenue, ok := results[0]["total_revenue"]; ok {
			return fmt.Sprintf("Total revenue from completed orders is %v. This represents the financial performance and should be monitored for growth trends.", revenue)
		}
	}

	return "Data retrieved successfully. Further analysis may be needed to extract specific business insights."
}

// fallbackSQLGeneration 後備 SQL 生成（當 LLM 不可用時使用）
func (m *MarketingQueryRunner) fallbackSQLGeneration(query string) (string, string) {
	queryLower := strings.ToLower(query)

	// 簡單的關鍵字匹配來生成 SQL
	if strings.Contains(queryLower, "how many customers") || strings.Contains(queryLower, "customer count") {
		return "SELECT COUNT(*) as customer_count FROM members", "Count of total customers in the system"
	}

	if strings.Contains(queryLower, "total orders") || strings.Contains(queryLower, "order count") || strings.Contains(queryLower, "how many orders") {
		return "SELECT COUNT(*) as order_count FROM orders", "Count of total orders in the system"
	}

	if strings.Contains(queryLower, "total revenue") || strings.Contains(queryLower, "revenue") {
		// 使用更通用的查詢，因為不知道確切的欄位名稱
		return "SELECT COUNT(*) as total_records FROM orders", "Total number of order records (revenue calculation requires specific column knowledge)"
	}

	if strings.Contains(queryLower, "top products") || strings.Contains(queryLower, "best selling") {
		return "SELECT COUNT(*) as product_count FROM products", "Count of products in the system"
	}

	if strings.Contains(queryLower, "recent orders") {
		return "SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5", "Most recent 5 orders by creation date"
	}

	// 默認查詢
	return "SELECT COUNT(*) as record_count FROM members", "Fallback query - total member count"
}

// SaveQueryResult 保存查詢結果
func (m *MarketingQueryRunner) SaveQueryResult(result *QueryResult) error {
	if m.knowledgeMgr == nil {
		return fmt.Errorf("knowledge manager not available")
	}

	// 存儲到向量數據庫
	queryKnowledge := map[string]interface{}{
		"phase":              "marketing_query",
		"description":        "Marketing query result with business insights",
		"query":              result.Query,
		"sql_query":          result.SQLQuery,
		"result_count":       len(result.Results),
		"has_error":          result.Error != "",
		"timestamp":          result.Timestamp,
		"insights_generated": result.BusinessInsights != "",
	}

	return m.knowledgeMgr.StorePhaseKnowledge("marketing_query", queryKnowledge)
}
