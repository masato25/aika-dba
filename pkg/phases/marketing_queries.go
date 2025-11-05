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
	"gopkg.in/yaml.v3"
)

// MarketingScenario 行銷分析場景
type MarketingScenario struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Category      string   `yaml:"category"`
	ExpectedTools []string `yaml:"expected_tools"`
}

// MarketingScenariosConfig 場景配置
type MarketingScenariosConfig struct {
	Scenarios []MarketingScenario `yaml:"scenarios"`
}

// MarketingQueryRunner 行銷查詢測試執行器
type MarketingQueryRunner struct {
	config    *config.Config
	db        *sql.DB
	llmClient *LLMClient
	client    *http.Client
}

// NewMarketingQueryRunner 創建行銷查詢測試執行器
func NewMarketingQueryRunner(cfg *config.Config, db *sql.DB) *MarketingQueryRunner {
	return &MarketingQueryRunner{
		config:    cfg,
		db:        db,
		llmClient: NewLLMClient(cfg),
		client:    &http.Client{},
	}
}

// RunScenario 執行單一場景分析
func (m *MarketingQueryRunner) RunScenario(scenarioName string) error {
	log.Printf("=== Running Marketing Scenario: %s ===", scenarioName)

	// 載入場景腳本
	scenarios, err := m.loadScenarios()
	if err != nil {
		return fmt.Errorf("failed to load scenarios: %v", err)
	}

	// 查找指定的場景
	var scenario *MarketingScenario
	for _, s := range scenarios {
		if s.Name == scenarioName {
			scenario = &s
			break
		}
	}

	if scenario == nil {
		return fmt.Errorf("scenario '%s' not found. Available scenarios: %v", scenarioName, m.getScenarioNames(scenarios))
	}

	log.Printf("Scenario: %s", scenario.Description)
	log.Printf("Category: %s", scenario.Category)

	// 使用LLM生成查詢
	sqlQuery, err := m.generateQueryWithLLM(scenario.Description)
	if err != nil {
		log.Printf("Failed to generate query for scenario: %v", err)
		return err
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

// Run 執行所有行銷查詢測試
func (m *MarketingQueryRunner) Run() error {
	log.Println("=== Starting Marketing Query Test with LLM ===")

	// 載入場景腳本
	scenarios, err := m.loadScenarios()
	if err != nil {
		return fmt.Errorf("failed to load scenarios: %v", err)
	}

	// 為每個場景生成LLM驅動的查詢
	for i, scenario := range scenarios {
		log.Printf("\n=== Marketing Scenario %d: %s ===", i+1, scenario.Name)
		log.Printf("Category: %s", scenario.Category)
		log.Printf("Description: %s", scenario.Description)

		// 使用LLM生成查詢
		sqlQuery, err := m.generateQueryWithLLM(scenario.Description)
		if err != nil {
			log.Printf("Failed to generate query for scenario: %v", err)
			continue
		}

		log.Printf("Generated SQL Query:\n%s", sqlQuery)

		// 執行查詢
		results, err := m.executeQuery(sqlQuery)
		if err != nil {
			log.Printf("Query execution failed: %v", err)
			continue
		}

		log.Printf("Results (%d rows):", len(results))
		m.displayResults(results)
		log.Println(strings.Repeat("-", 80))
	}

	log.Println("Marketing query test completed successfully")
	return nil
}

// generateQueryWithLLM 使用LLM生成查詢，包含工具調用功能
func (m *MarketingQueryRunner) generateQueryWithLLM(scenario string) (string, error) {
	prompt := fmt.Sprintf(`你是一個行銷分析專家，需要根據場景生成SQL查詢。

行銷分析場景：%s

你可以使用以下工具來獲取資料庫信息：
1. read_phase1_data - 讀取Phase 1統計分析結果 (knowledge/phase1_analysis.json)
2. read_phase2_data - 讀取Phase 2 AI分析結果 (knowledge/phase2_analysis.json) 
3. read_phase4_data - 讀取Phase 4維度建模結果 (knowledge/phase4_dimensions.json)

請根據場景需求，先決定需要查看哪些資料，然後生成最適合的SQL查詢。

請以JSON格式回應，包含以下字段：
- reasoning: 你的分析推理過程
- tools_to_use: 需要使用的工具列表 (例如: ["read_phase1_data", "read_phase2_data"])
- sql_query: 生成的SQL查詢語句

回應格式：
{
  "reasoning": "分析推理...",
  "tools_to_use": ["read_phase1_data", "read_phase2_data"],
  "sql_query": "SELECT ... FROM ..."
}`, scenario)

	// 使用工具調用的方式與LLM交互
	ctx := context.Background()

	// 首先請求LLM決定使用哪些工具
	toolResponse, err := m.callLLMWithTools(ctx, prompt)
	if err != nil {
		log.Printf("LLM tool call failed, using fallback: %v", err)
		return m.generateFallbackQuery(scenario), nil
	}

	// 解析LLM回應
	var llmResponse struct {
		Reasoning  string   `json:"reasoning"`
		ToolsToUse []string `json:"tools_to_use"`
		SQLQuery   string   `json:"sql_query"`
	}

	if err := json.Unmarshal([]byte(toolResponse), &llmResponse); err != nil {
		log.Printf("Failed to parse LLM tool response: %v", err)
		return m.generateFallbackQuery(scenario), nil
	}

	log.Printf("LLM Reasoning: %s", llmResponse.Reasoning)
	log.Printf("Tools to use: %v", llmResponse.ToolsToUse)

	// 如果LLM指定了需要使用的工具，執行它們
	if len(llmResponse.ToolsToUse) > 0 {
		toolData := m.executeTools(llmResponse.ToolsToUse)
		log.Printf("Tool execution results: %v", toolData)
		// 在實際實現中，可以將toolData傳回給LLM進行進一步處理
	}

	return llmResponse.SQLQuery, nil
}

// callLLMWithTools 調用LLM並處理工具調用
func (m *MarketingQueryRunner) callLLMWithTools(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": m.config.LLM.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一個行銷分析專家，可以使用工具來獲取資料庫信息。請根據用戶需求決定使用哪些工具，然後生成SQL查詢。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
		"max_tokens":  2000,
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

	return content, nil
}

// executeTools 執行指定的工具
func (m *MarketingQueryRunner) executeTools(toolNames []string) map[string]interface{} {
	results := make(map[string]interface{})

	for _, toolName := range toolNames {
		switch toolName {
		case "read_phase1_data":
			data, err := m.readPhase1Data()
			if err != nil {
				log.Printf("Failed to read phase1 data: %v", err)
				results["phase1"] = "error reading data"
			} else {
				results["phase1"] = data
			}
		case "read_phase2_data":
			data, err := m.readPhase2Data()
			if err != nil {
				log.Printf("Failed to read phase2 data: %v", err)
				results["phase2"] = "error reading data"
			} else {
				results["phase2"] = data
			}
		case "read_phase4_data":
			data, err := m.readPhase4Data()
			if err != nil {
				log.Printf("Failed to read phase4 data: %v", err)
				results["phase4"] = "error reading data"
			} else {
				results["phase4"] = data
			}
		default:
			log.Printf("Unknown tool: %s", toolName)
		}
	}

	return results
}

// readPhase1Data 讀取Phase 1數據
func (m *MarketingQueryRunner) readPhase1Data() (interface{}, error) {
	data, err := os.ReadFile("knowledge/phase1_analysis.json")
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// readPhase2Data 讀取Phase 2數據
func (m *MarketingQueryRunner) readPhase2Data() (interface{}, error) {
	data, err := os.ReadFile("knowledge/phase2_analysis.json")
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// readPhase4Data 讀取Phase 4數據
func (m *MarketingQueryRunner) readPhase4Data() (interface{}, error) {
	data, err := os.ReadFile("knowledge/phase4_dimensions.json")
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// generateFallbackQuery 生成後備查詢
func (m *MarketingQueryRunner) generateFallbackQuery(scenario string) string {
	switch scenario {
	case "分析最暢銷的產品及其收入表現":
		return "SELECT p.name, SUM(oi.quantity) as total_sold, SUM(oi.total) as revenue FROM products p JOIN order_items oi ON p.id = oi.product_id GROUP BY p.id, p.name ORDER BY revenue DESC LIMIT 10;"
	case "根據購買頻率對客戶進行分群分析":
		return "SELECT CASE WHEN total_orders >= 10 THEN 'High' WHEN total_orders >= 5 THEN 'Medium' ELSE 'Low' END as segment, COUNT(*) as count FROM customers GROUP BY segment;"
	case "分析月度銷售趨勢和季節性變化":
		return "SELECT DATE_TRUNC('month', ordered_at) as month, SUM(total) as revenue FROM orders GROUP BY month ORDER BY month DESC LIMIT 12;"
	case "評估產品分類的銷售績效":
		return "SELECT c.name, SUM(oi.total) as revenue FROM categories c JOIN products p ON c.id = p.category_id JOIN order_items oi ON p.id = oi.product_id GROUP BY c.id, c.name ORDER BY revenue DESC;"
	case "計算客戶終身價值和忠誠度分析":
		return "SELECT name, total_spent, total_orders, registration_date FROM customers ORDER BY total_spent DESC LIMIT 20;"
	case "分析地理銷售分佈和區域表現":
		return "SELECT shipping_address->>'country' as country, SUM(total) as revenue FROM orders GROUP BY country ORDER BY revenue DESC LIMIT 10;"
	case "評估促銷活動的有效性和ROI":
		return "SELECT code, discount_value, usage_count FROM coupons ORDER BY usage_count DESC LIMIT 10;"
	default:
		return "SELECT 'No query available for this scenario' as message;"
	}
}

// MarketingQuery 行銷查詢結構
type MarketingQuery struct {
	Purpose string
	SQL     string
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

// loadScenarios 載入場景腳本
func (m *MarketingQueryRunner) loadScenarios() ([]MarketingScenario, error) {
	data, err := os.ReadFile("scripts/marketing_scenarios.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read scenarios file: %v", err)
	}

	var config MarketingScenariosConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse scenarios file: %v", err)
	}

	return config.Scenarios, nil
}

// getScenarioNames 獲取所有場景名稱
func (m *MarketingQueryRunner) getScenarioNames(scenarios []MarketingScenario) []string {
	names := make([]string, len(scenarios))
	for i, s := range scenarios {
		names[i] = s.Name
	}
	return names
}
