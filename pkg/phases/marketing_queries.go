package phases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// MarketingScenario 行銷分析場景 (保留結構體定義以防將來需要)
type MarketingScenario struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Category      string   `yaml:"category"`
	ExpectedTools []string `yaml:"expected_tools"`
}

// MarketingScenariosConfig 場景配置 (保留結構體定義以防將來需要)
type MarketingScenariosConfig struct {
	Scenarios []MarketingScenario `yaml:"scenarios"`
}

// MarketingQueryRunner 行銷查詢測試執行器
type MarketingQueryRunner struct {
	config           *config.Config
	db               *sql.DB
	llmClient        *LLMClient
	client           *http.Client
	knowledgeIndexer *vectorstore.KnowledgeIndexer
}

// NewMarketingQueryRunner 創建行銷查詢測試執行器
func NewMarketingQueryRunner(cfg *config.Config, db *sql.DB) *MarketingQueryRunner {
	// 初始化嵌入生成器
	var embedder vectorstore.Embedder

	switch cfg.VectorStore.EmbedderType {
	case "qwen":
		embedder = vectorstore.NewQwenEmbedder(cfg.VectorStore.QwenModelPath, cfg.VectorStore.EmbeddingDimension)
		log.Printf("Initialized Qwen embedder with dimension %d", cfg.VectorStore.EmbeddingDimension)
	case "simple":
		embedder = vectorstore.NewSimpleHashEmbedder(cfg.VectorStore.EmbeddingDimension)
		log.Printf("Using simple hash embedder with dimension %d", cfg.VectorStore.EmbeddingDimension)
	default:
		log.Printf("Unknown embedder type: %s, using simple hash", cfg.VectorStore.EmbedderType)
		embedder = vectorstore.NewSimpleHashEmbedder(cfg.VectorStore.EmbeddingDimension)
	}

	// 初始化向量索引器
	vectorDBPath := cfg.VectorStore.DatabasePath
	indexer, err := vectorstore.NewKnowledgeIndexer(vectorDBPath, embedder)
	if err != nil {
		log.Printf("Warning: Failed to initialize knowledge indexer: %v", err)
		log.Printf("Falling back to traditional knowledge loading")
		indexer = nil
	} else {
		// 檢查是否需要索引知識庫
		if stats, err := indexer.GetStats(); err == nil {
			if totalChunks, ok := stats["total_chunks"].(int); ok && totalChunks == 0 {
				// 索引知識庫
				knowledgeDir := "knowledge"
				if err := indexer.IndexKnowledgeBase(knowledgeDir); err != nil {
					log.Printf("Warning: Failed to index knowledge base: %v", err)
				} else {
					log.Printf("Knowledge base indexed successfully")
				}
			}
		}
	}

	return &MarketingQueryRunner{
		config:           cfg,
		db:               db,
		llmClient:        NewLLMClient(cfg),
		client:           &http.Client{},
		knowledgeIndexer: indexer,
	}
}

// RunScenario 執行單一場景分析
func (m *MarketingQueryRunner) RunScenario(scenarioName string) error {
	log.Printf("=== Running Marketing Scenario: %s ===", scenarioName)

	// 檢查是否為A:或Q:模式
	if strings.HasPrefix(strings.ToUpper(scenarioName), "A:") {
		// A模式：詢問資料庫資訊，不產生SQL
		return m.runAnalysisMode(strings.TrimSpace(scenarioName[2:]))
	} else if strings.HasPrefix(strings.ToUpper(scenarioName), "Q:") {
		// Q模式：直接執行SQL查詢
		return m.runQueryMode(strings.TrimSpace(scenarioName[2:]))
	} else {
		// 舊模式：使用LLM生成查詢
		log.Printf("Custom Scenario Description: %s", scenarioName)
		log.Printf("Using LLM with knowledge base for query generation...")

		// 使用LLM生成查詢
		sqlQuery, err := m.generateQueryWithLLM(scenarioName)
		if err != nil {
			log.Printf("LLM query generation failed, falling back to intelligent generation: %v", err)
			sqlQuery = m.generateIntelligentFallbackQuery(scenarioName)
		}

		log.Printf("Generated SQL Query:\n%s", sqlQuery)

		// 執行查詢
		results, err := m.executeQuery(sqlQuery)
		if err != nil {
			log.Printf("Query execution failed: %v", err)
			return err
		}

		log.Printf("Results (%d rows):", len(results))
		m.displayResults(results)

		log.Printf("Marketing scenario '%s' completed successfully", scenarioName)
		return nil
	}
}

// runAnalysisMode A模式：詢問資料庫資訊，不產生SQL
func (m *MarketingQueryRunner) runAnalysisMode(query string) error {
	log.Printf("=== Running Analysis Mode (A:) ===")
	log.Printf("Query: %s", query)

	// 載入相關知識庫內容
	knowledge, err := m.loadKnowledgeWithVectorSearch(query)
	if err != nil {
		log.Printf("Vector search failed, falling back to full knowledge load: %v", err)
		knowledge, err = m.loadKnowledgeBase()
		if err != nil {
			return fmt.Errorf("failed to load knowledge base: %v", err)
		}
	}

	// 構建分析提示
	prompt := fmt.Sprintf(`你是一個資深資料庫分析專家。用戶詢問關於資料庫的相關資訊，請基於提供的知識庫內容進行詳細解答。

用戶問題：%s

知識庫內容：
%s

請提供詳細的解答，包括：
1. 相關的資料庫表格和欄位說明
2. 資料結構和業務邏輯解釋
3. 可能的分析洞察或建議
4. 如果適用，說明如何進行相關的資料查詢

請用清晰易懂的方式回答，不要提及任何SQL語法或查詢細節。`, query, knowledge)

	// 使用LLM生成回答
	ctx := context.Background()

	requestBody := map[string]interface{}{
		"model": m.config.LLM.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一個資料庫分析專家，請基於知識庫內容回答用戶的資料庫相關問題。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  1500,
	}

	// 發送請求到LLM
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", m.config.LLM.Host, m.config.LLM.Port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if m.config.LLM.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.config.LLM.APIKey)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return fmt.Errorf("invalid response format: no choices")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format: invalid choice")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format: no message")
	}

	content, ok := message["content"].(string)
	if !ok {
		return fmt.Errorf("invalid response format: no content")
	}

	// 顯示回答
	log.Printf("=== Analysis Result ===")
	fmt.Println(content)

	log.Printf("Analysis mode completed successfully")
	return nil
}

// runQueryMode Q模式：直接執行SQL查詢
func (m *MarketingQueryRunner) runQueryMode(sqlQuery string) error {
	log.Printf("=== Running Query Mode (Q:) ===")
	log.Printf("SQL Query: %s", sqlQuery)

	// 驗證SQL查詢（基本檢查）
	if strings.TrimSpace(strings.ToUpper(sqlQuery)) == "" {
		return fmt.Errorf("SQL query cannot be empty")
	}

	// 檢查是否包含危險的SQL關鍵字（簡單的安全檢查）
	dangerousKeywords := []string{"DROP", "DELETE", "UPDATE", "INSERT", "ALTER", "CREATE", "TRUNCATE"}
	upperQuery := strings.ToUpper(sqlQuery)
	for _, keyword := range dangerousKeywords {
		if strings.Contains(upperQuery, keyword) {
			log.Printf("Warning: Query contains potentially dangerous keyword: %s", keyword)
			log.Printf("Please ensure this is intentional and safe to execute")
		}
	}

	// 執行查詢
	results, err := m.executeQuery(sqlQuery)
	if err != nil {
		log.Printf("Query execution failed: %v", err)
		return err
	}

	log.Printf("Results (%d rows):", len(results))
	m.displayResults(results)

	log.Printf("Query mode completed successfully")
	return nil
}

// generateQueryWithLLM 使用LLM生成查詢，包含知識庫支持
func (m *MarketingQueryRunner) generateQueryWithLLM(scenario string) (string, error) {
	// 使用向量存儲搜索相關知識
	knowledge, err := m.loadKnowledgeWithVectorSearch(scenario)
	if err != nil {
		log.Printf("Vector search failed, falling back to full knowledge load: %v", err)
		knowledge, err = m.loadKnowledgeBase()
		if err != nil {
			return "", err
		}
	}

	prompt := fmt.Sprintf(`你是一個資深行銷分析專家，需要根據用戶需求和資料庫知識生成PostgreSQL SQL查詢。

用戶分析需求：%s

資料庫知識庫內容：
%s

請根據用戶的需求和知識庫中的資料結構信息，生成最適合的PostgreSQL SQL查詢。

重要注意事項：
1. 使用PostgreSQL語法，不要使用MySQL語法
2. 根據知識庫中的表格結構來決定合適的查詢，特別注意表名和欄位名稱
3. 確保生成的SQL語法正確且安全
4. 如果需要聯表查詢，請根據外鍵關係進行JOIN
5. 優先使用知識庫中明確提到的表和欄位
6. 如果知識庫中沒有相關信息，請使用常見的電商資料庫表結構

請直接返回可執行的PostgreSQL SQL查詢語句，不要包含任何解釋或額外文字。`, scenario, knowledge)

	// 使用LLM生成查詢
	ctx := context.Background()

	requestBody := map[string]interface{}{
		"model": m.config.LLM.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一個PostgreSQL專家，根據用戶需求和資料庫知識生成準確的SQL查詢。只返回SQL語句，不要解釋。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.1, // 降低溫度以獲得更準確的結果
		"max_tokens":  1000,
	}

	// 發送請求到LLM
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", m.config.LLM.Host, m.config.LLM.Port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if m.config.LLM.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.config.LLM.APIKey)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("invalid response format: no choices")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format: invalid choice")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format: no message")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response format: no content")
	}

	// 清理回應，只保留SQL部分
	sqlQuery := strings.TrimSpace(content)
	// 移除可能的markdown代碼塊標記
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimPrefix(sqlQuery, "```")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	// 驗證生成的SQL
	if err := m.validateSQLQuery(sqlQuery); err != nil {
		log.Printf("SQL驗證失敗: %v", err)
		// 不返回錯誤，只記錄警告
	}

	return sqlQuery, nil
}

// loadKnowledgeBase 載入知識庫內容
func (m *MarketingQueryRunner) loadKnowledgeBase() (string, error) {
	var knowledge strings.Builder

	// 讀取Phase 1分析結果
	phase1Data, err := os.ReadFile("knowledge/phase1_analysis.json")
	if err == nil {
		knowledge.WriteString("=== Phase 1 統計分析結果 ===\n")
		knowledge.Write(phase1Data)
		knowledge.WriteString("\n\n")
	} else {
		log.Printf("Warning: Could not read phase1_analysis.json: %v", err)
	}

	// 讀取Phase 2分析結果
	phase2Data, err := os.ReadFile("knowledge/phase2_analysis.json")
	if err == nil {
		knowledge.WriteString("=== Phase 2 AI分析結果 ===\n")
		knowledge.Write(phase2Data)
		knowledge.WriteString("\n\n")
	} else {
		log.Printf("Warning: Could not read phase2_analysis.json: %v", err)
	}

	// 讀取Phase 4維度建模結果
	phase4Data, err := os.ReadFile("knowledge/phase4_dimensions.json")
	if err == nil {
		knowledge.WriteString("=== Phase 4 維度建模結果 ===\n")
		knowledge.Write(phase4Data)
		knowledge.WriteString("\n\n")
	} else {
		log.Printf("Warning: Could not read phase4_dimensions.json: %v", err)
	}

	if knowledge.Len() == 0 {
		return "", fmt.Errorf("no knowledge base files found")
	}

	return knowledge.String(), nil
}

// loadKnowledgeWithVectorSearch 使用向量搜索載入相關知識
func (m *MarketingQueryRunner) loadKnowledgeWithVectorSearch(scenario string) (string, error) {
	if m.knowledgeIndexer == nil {
		return "", fmt.Errorf("knowledge indexer not initialized")
	}

	// 搜索相關知識塊
	results, err := m.knowledgeIndexer.SearchKnowledge(scenario, 5) // 獲取前5個最相關的塊
	if err != nil {
		return "", fmt.Errorf("failed to search knowledge: %v", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no relevant knowledge found")
	}

	// 組合相關知識
	var knowledge strings.Builder
	knowledge.WriteString("=== 相關資料庫知識 ===\n")

	// 如果沒有找到相關知識，提供基本的表結構信息
	if len(results) == 0 || !containsLoyaltyPointsInfo(results) {
		log.Printf("No loyalty_points knowledge found, providing fallback table information")
		knowledge.WriteString("主要業務表結構:\n")
		knowledge.WriteString("- customers: 客戶信息 (id, email, name, total_spent, total_orders, loyalty_points, registration_date)\n")
		knowledge.WriteString("- orders: 訂單信息 (id, customer_id, total, ordered_at, status)\n")
		knowledge.WriteString("- products: 產品信息 (id, name, price, category_id)\n")
		knowledge.WriteString("- order_items: 訂單項目 (order_id, product_id, quantity, price)\n")
		knowledge.WriteString("- payments: 支付信息 (order_id, amount, payment_method, status)\n\n")
	} else {
		for i, result := range results {
			knowledge.WriteString(fmt.Sprintf("知識塊 %d:\n", i+1))
			knowledge.WriteString(result.Content)
			knowledge.WriteString("\n\n")
		}
	}

	log.Printf("Retrieved %d relevant knowledge chunks using vector search", len(results))
	// 調試：記錄知識內容
	log.Printf("Knowledge content preview: %s", knowledge.String()[:min(500, len(knowledge.String()))])

	return knowledge.String(), nil
}

// generateIntelligentFallbackQuery 提供簡單的LLM查詢生成，包含基本表結構信息
func (m *MarketingQueryRunner) generateIntelligentFallbackQuery(scenario string) string {
	// 優先使用向量搜索載入知識庫
	knowledge, err := m.loadKnowledgeWithVectorSearch(scenario)
	if err != nil {
		log.Printf("Vector search failed in fallback, using full knowledge: %v", err)
		knowledge, err = m.loadKnowledgeBase()
		if err != nil {
			log.Printf("Failed to load knowledge base: %v", err)
			knowledge = "知識庫載入失敗，使用基本表結構信息。"
		}
	}

	// 提供基本的表結構信息（如果知識庫太大或失敗）
	basicSchema := `
主要表格：
- customers: 客戶信息 (id, name, email, total_spent, total_orders, gender, registration_date)
- orders: 訂單信息 (id, customer_id, total, ordered_at, status)
- order_items: 訂單項目 (order_id, product_id, quantity, price, total)
- products: 產品信息 (id, name, price, category_id)
- categories: 產品分類 (id, name)

時間欄位使用 timestamp with time zone 類型。
`

	contextInfo := knowledge
	if len(knowledge) > 2000 { // 如果知識庫太大，使用基本信息
		contextInfo = basicSchema
	}

	prompt := fmt.Sprintf(`根據用戶的行銷分析需求和資料庫結構生成PostgreSQL SQL查詢。

需求：%s

資料庫結構信息：
%s

請生成一個簡單的PostgreSQL查詢來滿足這個分析需求。只返回SQL語句，不要解釋。`, scenario, contextInfo)

	ctx := context.Background()

	requestBody := map[string]interface{}{
		"model": "test",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.1,
		"max_tokens":  300,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Failed to marshal request: %v", err)
		return fmt.Sprintf("SELECT '無法生成查詢：%s' as error;", scenario)
	}

	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", m.config.LLM.Host, m.config.LLM.Port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return fmt.Sprintf("SELECT '請求失敗：%s' as error;", scenario)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		log.Printf("LLM request failed: %v", err)
		return fmt.Sprintf("SELECT 'LLM服務不可用：%s' as error;", scenario)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("LLM request returned status: %d", resp.StatusCode)
		return fmt.Sprintf("SELECT 'LLM服務錯誤 %d：%s' as error;", resp.StatusCode, scenario)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Failed to decode response: %v", err)
		return fmt.Sprintf("SELECT '響應解析失敗：%s' as error;", scenario)
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		log.Printf("No choices in response")
		return fmt.Sprintf("SELECT '無效響應：%s' as error;", scenario)
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		log.Printf("Invalid choice format")
		return fmt.Sprintf("SELECT '無效選擇：%s' as error;", scenario)
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		log.Printf("No message in choice")
		return fmt.Sprintf("SELECT '無消息：%s' as error;", scenario)
	}

	content, ok := message["content"].(string)
	if !ok {
		log.Printf("No content in message")
		return fmt.Sprintf("SELECT '無內容：%s' as error;", scenario)
	}

	// 清理和驗證SQL
	sqlQuery := strings.TrimSpace(content)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimPrefix(sqlQuery, "```")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	// 基本驗證：確保包含SELECT
	if !strings.Contains(strings.ToUpper(sqlQuery), "SELECT") {
		log.Printf("Generated content is not a SQL query: %s", sqlQuery)
		return fmt.Sprintf("SELECT '生成的不是SQL查詢：%s' as error;", scenario)
	}

	log.Printf("Successfully generated SQL with knowledge base context")
	return sqlQuery
}

// validateSQLQuery 驗證SQL查詢是否使用了有效的表和欄位
func (m *MarketingQueryRunner) validateSQLQuery(sql string) error {
	// 載入表結構信息
	phase1Data, err := os.ReadFile("knowledge/phase1_analysis.json")
	if err != nil {
		return fmt.Errorf("無法讀取表結構信息: %v", err)
	}

	var dbInfo map[string]interface{}
	if err := json.Unmarshal(phase1Data, &dbInfo); err != nil {
		return fmt.Errorf("無法解析表結構信息: %v", err)
	}

	tables, ok := dbInfo["tables"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("表結構信息格式錯誤")
	}

	// 提取SQL中的表名和欄位名（簡單的正則表達式解析）
	tableNames := m.extractTableNamesFromSQL(sql)
	columnNames := m.extractColumnNamesFromSQL(sql)

	// 驗證表名
	for _, tableName := range tableNames {
		if _, exists := tables[tableName]; !exists {
			log.Printf("警告: SQL中使用了不存在的表 '%s'", tableName)
			// 不返回錯誤，只記錄警告，因為LLM可能使用別名或子查詢
		}
	}

	// 驗證欄位名（更寬鬆的檢查）
	for _, colName := range columnNames {
		found := false
		for _, tableInfo := range tables {
			if tableData, ok := tableInfo.(map[string]interface{}); ok {
				if schemaArray, ok := tableData["schema"].([]interface{}); ok {
					for _, colInterface := range schemaArray {
						if colMap, ok := colInterface.(map[string]interface{}); ok {
							if colMap["name"] == colName {
								found = true
								break
							}
						}
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			log.Printf("警告: SQL中使用了可能不存在的欄位 '%s'", colName)
		}
	}

	return nil
}

// extractTableNamesFromSQL 從SQL中提取表名（簡單實現）
func (m *MarketingQueryRunner) extractTableNamesFromSQL(sql string) []string {
	var tables []string

	// 簡單的FROM子句解析
	upperSQL := strings.ToUpper(sql)
	fromIndex := strings.Index(upperSQL, "FROM")
	if fromIndex == -1 {
		return tables
	}

	fromClause := sql[fromIndex+4:]
	// 分割WHERE, GROUP BY, ORDER BY等子句
	clauses := []string{"WHERE", "GROUP BY", "ORDER BY", "HAVING", "LIMIT", "JOIN"}
	minIndex := len(fromClause)
	for _, clause := range clauses {
		if idx := strings.Index(strings.ToUpper(fromClause), clause); idx != -1 && idx < minIndex {
			minIndex = idx
		}
	}

	if minIndex < len(fromClause) {
		fromClause = fromClause[:minIndex]
	}

	// 簡單提取表名（處理JOIN和逗號分隔）
	parts := strings.Fields(fromClause)
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, ","))
		if part != "" && !strings.Contains(part, "(") && !strings.Contains(part, ")") {
			// 移除別名
			if idx := strings.Index(strings.ToUpper(part), " AS "); idx != -1 {
				part = part[:idx]
			} else if idx := strings.LastIndex(part, " "); idx != -1 {
				part = part[:idx]
			}
			if part != "" && part != "JOIN" && part != "INNER" && part != "LEFT" && part != "RIGHT" && part != "FULL" {
				tables = append(tables, strings.ToLower(part))
			}
		}
	}

	return tables
}

// extractColumnNamesFromSQL 從SQL中提取欄位名（簡單實現）
func (m *MarketingQueryRunner) extractColumnNamesFromSQL(sql string) []string {
	var columns []string

	// 簡單的SELECT子句解析
	upperSQL := strings.ToUpper(sql)
	selectIndex := strings.Index(upperSQL, "SELECT")
	if selectIndex == -1 {
		return columns
	}

	selectClause := sql[selectIndex+6:]
	fromIndex := strings.Index(strings.ToUpper(selectClause), "FROM")
	if fromIndex == -1 {
		return columns
	}

	selectClause = selectClause[:fromIndex]

	// 分割欄位
	parts := strings.Split(selectClause, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// 移除聚合函數和別名
		if idx := strings.Index(strings.ToUpper(part), " AS "); idx != -1 {
			part = part[:idx]
		} else if idx := strings.LastIndex(part, " "); idx != -1 && strings.Contains("SUM COUNT AVG MIN MAX", strings.ToUpper(part[:idx])) {
			// 保留聚合函數中的欄位名
		}
		// 提取欄位名（簡單處理點號分隔）
		if strings.Contains(part, ".") {
			dotParts := strings.Split(part, ".")
			if len(dotParts) == 2 {
				colName := strings.TrimSpace(dotParts[1])
				if colName != "*" && colName != "" {
					columns = append(columns, strings.ToLower(colName))
				}
			}
		} else if part != "*" && !strings.Contains(strings.ToUpper(part), "COUNT(*)") {
			// 簡單的欄位名
			colName := strings.TrimSpace(strings.ToLower(part))
			if colName != "" && !strings.Contains(colName, "(") {
				columns = append(columns, colName)
			}
		}
	}

	return columns
}

// executeQuery 執行SQL查詢
func (m *MarketingQueryRunner) executeQuery(sql string) ([]map[string]interface{}, error) {
	rows, err := m.db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
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
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	return results, nil
}

// displayResults 顯示查詢結果
func (m *MarketingQueryRunner) displayResults(results []map[string]interface{}) {
	if len(results) == 0 {
		fmt.Println("No results found")
		return
	}

	// 獲取所有欄位名稱
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	// 顯示表頭
	fmt.Printf("| %-20s ", "Column")
	for _, col := range columns {
		fmt.Printf("| %-20s ", truncateString(col, 20))
	}
	fmt.Println("|")
	fmt.Println(strings.Repeat("-", 21*len(columns)+1))

	// 顯示資料
	for i, row := range results {
		if i >= 10 { // 只顯示前10行
			fmt.Printf("... and %d more rows\n", len(results)-10)
			break
		}
		fmt.Printf("| %-20s ", "Row")
		for _, col := range columns {
			value := row[col]
			if value == nil {
				fmt.Printf("| %-20s ", "NULL")
			} else {
				fmt.Printf("| %-20s ", truncateString(fmt.Sprintf("%v", value), 20))
			}
		}
		fmt.Println("|")
	}
}

// truncateString 截斷字串
func truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-3] + "..."
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// containsLoyaltyPointsInfo 檢查知識塊是否包含loyalty_points信息
func containsLoyaltyPointsInfo(results []vectorstore.KnowledgeResult) bool {
	for _, result := range results {
		contentLower := strings.ToLower(result.Content)
		if strings.Contains(contentLower, "loyalty_points") {
			return true
		}
	}
	return false
}

// Run 執行所有行銷查詢測試
func (m *MarketingQueryRunner) Run() error {
	log.Println("=== Starting Marketing Query Test with Intelligent Generation ===")
	log.Println("Note: This system now supports two modes:")
	log.Println("  A: [description] - Ask questions about database structure and information (no SQL execution)")
	log.Println("  Q: [SQL query]   - Execute direct SQL queries")
	log.Println("  [description]    - Generate SQL queries using AI (legacy mode)")
	log.Println("")
	log.Println("Usage examples:")
	log.Println("  go run cmd/main.go -command marketing -scenario \"A: 請說明loyalty_points欄位的用途\"")
	log.Println("  go run cmd/main.go -command marketing -scenario \"Q: SELECT COUNT(*) FROM customers WHERE loyalty_points > 100\"")
	log.Println("  go run cmd/main.go -command marketing -scenario \"分析客戶忠誠度分佈\"")

	// 提供一些示例場景
	examples := []string{
		"A: 請說明資料庫中有哪些客戶相關的欄位",
		"Q: SELECT COUNT(*) FROM customers WHERE total_spent > 1000",
		"分析最暢銷的產品及其收入表現",
		"根據購買頻率對客戶進行分群分析",
		"分析月度銷售趨勢和季節性變化",
	}

	log.Println("")
	log.Println("Example scenarios you can try:")
	for i, example := range examples {
		log.Printf("  %d. %s", i+1, example)
	}

	log.Println("")
	log.Println("Marketing query system is ready for arbitrary analysis descriptions")
	return nil
}
