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
	phase1Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, 1)
	if err == nil {
		for _, result := range phase1Results {
			// 截斷知識內容以適應 LLM 上下文限制
			truncatedContent := result.Content
			if len(truncatedContent) > 500 {
				truncatedContent = truncatedContent[:500] + "..."
			}
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 1 - Schema Analysis: %s", truncatedContent))
		}
	}

	// 檢索 Phase 2 知識 (業務邏輯分析)
	phase2Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, 1)
	if err == nil {
		for _, result := range phase2Results {
			// 截斷知識內容以適應 LLM 上下文限制
			truncatedContent := result.Content
			if len(truncatedContent) > 500 {
				truncatedContent = truncatedContent[:500] + "..."
			}
			allKnowledge = append(allKnowledge, fmt.Sprintf("Phase 2 - Business Logic: %s", truncatedContent))
		}
	}

	// 檢索 Phase 3 知識 (商業邏輯描述)
	phase3Results, err := m.knowledgeMgr.RetrievePhaseKnowledge("phase3", query, 1)
	if err == nil {
		for _, result := range phase3Results {
			// 截斷知識內容以適應 LLM 上下文限制
			truncatedContent := result.Content
			if len(truncatedContent) > 500 {
				truncatedContent = truncatedContent[:500] + "..."
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

	// 構造 LLM 提示 - 使用向量知識輔助，但不強制依賴
	prompt := fmt.Sprintf(`You are a SQL expert analyzing an e-commerce database. Use the business knowledge when available, but generate reasonable SQL based on common e-commerce patterns when knowledge is limited.

Database Schema:
%s

Business Knowledge from Vector Database (use when helpful, but don't rely on it exclusively):
%s

User Question: %s

COMMON E-COMMERCE PATTERNS TO CONSIDER:
- Products table typically contains: id, name, price, inventory_quantity, category_id, is_active
- Sales data might be in: product_sales_summary (total_sold, revenue), order_items (quantity), orders (order_date)
- Customer data: customers, customer_addresses
- Categories: categories
- "Best-selling" or "popular" products: sort by total_sold from product_sales_summary, or count from order_items
- Active products have is_active = true
- Use JOINs: products.id = product_sales_summary.product_id or products.id = order_items.product_id

REQUIREMENTS:
1. Generate ONLY SELECT queries that directly answer the user's question
2. Use EXACTLY the table names and column names found in the Database Schema above
3. Do NOT invent column names - use only the columns listed in the schema
4. For "best-selling/popular products": look for sales/order data, sort by quantity sold
5. Include appropriate JOINs, WHERE, GROUP BY, ORDER BY clauses as needed
6. Limit results to maximum 50 rows for performance
7. When in doubt, make reasonable assumptions based on common e-commerce database patterns

Return ONLY the SQL query without any explanations or markdown formatting:`, schemaInfo, relevantKnowledge, naturalLanguageQuery)

	// 調用 LLM 生成 SQL
	response, err := m.llmClient.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		// 如果 LLM 完全失敗，返回錯誤而不是使用寫死 SQL
		return "", "", fmt.Errorf("LLM failed to generate SQL query: %v. Business knowledge may be insufficient or LLM service unavailable", err)
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

	explanation := fmt.Sprintf("SQL query generated by LLM using business knowledge from vector database to answer: %s", naturalLanguageQuery)

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

// generateBusinessInsights 生成業務洞察
func (m *MarketingQueryRunner) generateBusinessInsights(query string, results []map[string]interface{}, knowledge string) (string, error) {
	if len(results) == 0 {
		return "沒有可用的數據來生成洞察。", nil
	}

	// 構造 LLM 提示
	resultSummary := fmt.Sprintf("查詢返回了 %d 行數據，包含 %d 個欄位", len(results), len(results[0]))

	prompt := fmt.Sprintf(`你是一位商業分析師。根據以下查詢、結果和來自向量數據庫的商業知識，提供關鍵商業洞察。

用戶查詢：%s

查詢結果摘要：%s

來自向量數據庫的商業知識：%s

請提供：
1. 數據中的關鍵發現
2. 商業影響
3. 行動建議
4. 觀察到的趨勢或模式

請用中文回應，保持簡潔但有洞察力。重點關注可操作的商業洞察。`, query, resultSummary, knowledge)

	// 調用 LLM 生成洞察
	response, err := m.llmClient.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		// 如果 LLM 失敗，提供基本的結果摘要
		log.Printf("LLM failed for insights: %v", err)
		return fmt.Sprintf("查詢執行成功，返回 %d 個結果。LLM 分析不可用。", len(results)), nil
	}

	return strings.TrimSpace(response), nil
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
