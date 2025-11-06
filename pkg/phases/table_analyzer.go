package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// LLMAnalysisResult LLM 分析結果
type LLMAnalysisResult struct {
	TableName string    `json:"table_name"`
	Analysis  string    `json:"analysis"`
	Timestamp time.Time `json:"timestamp"`
}

// TableAnalysisTask 表格分析任務
type TableAnalysisTask struct {
	TableName string
	Status    string // "pending", "in_progress", "completed", "failed"
	Result    *LLMAnalysisResult
	Error     error
	Priority  int // 0 = highest, higher number = lower priority
}

// TableAnalysisOrchestrator 表格分析協調器
type TableAnalysisOrchestrator struct {
	config       *config.Config
	reader       *Phase1ResultReader
	tasks        []*TableAnalysisTask
	llmClient    *LLMClient
	mcpServer    MCPServer
	currentTask  *TableAnalysisTask
	results      map[string]*LLMAnalysisResult
	knowledgeMgr *vectorstore.KnowledgeManager
}

// NewTableAnalysisOrchestrator 創建表格分析協調器
func NewTableAnalysisOrchestrator(cfg *config.Config, reader *Phase1ResultReader, mcpServer MCPServer, knowledgeMgr *vectorstore.KnowledgeManager) *TableAnalysisOrchestrator {
	return &TableAnalysisOrchestrator{
		config:       cfg,
		reader:       reader,
		tasks:        make([]*TableAnalysisTask, 0),
		llmClient:    NewLLMClient(cfg),
		mcpServer:    mcpServer,
		results:      make(map[string]*LLMAnalysisResult),
		knowledgeMgr: knowledgeMgr,
	}
}

// InitializeTasks 初始化分析任務
func (o *TableAnalysisOrchestrator) InitializeTasks() error {
	// 優先使用文件讀取器獲取表格名稱，因為向量存儲的解析可能不完整
	log.Printf("Initializing analysis tasks using file-based reader")
	tableNames, err := o.reader.GetTableNames()
	if err != nil {
		log.Printf("Failed to get table names from file reader: %v", err)
		log.Printf("Attempting to retrieve from vector store as fallback")
		// 後備方案：從向量存儲檢索
		query := "database tables analysis schema columns constraints"
		results, err := o.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, 10)
		if err != nil {
			return fmt.Errorf("failed to get table names from both file reader and vector store: %v", err)
		}

		if len(results) == 0 {
			return fmt.Errorf("no table information found in either file reader or vector store")
		}

		// 從檢索到的知識中提取表格名稱
		tableNames = o.extractTableNamesFromKnowledge(results)
	}

	log.Printf("Initializing analysis tasks for %d tables", len(tableNames))
	o.initializeTasksFromNames(tableNames)
	return nil
}

// initializeTasksFromNames 從表格名稱列表初始化任務
func (o *TableAnalysisOrchestrator) initializeTasksFromNames(tableNames []string) {
	log.Printf("Initializing analysis tasks for %d tables from file", len(tableNames))

	for _, tableName := range tableNames {
		task := &TableAnalysisTask{
			TableName: tableName,
			Status:    "pending",
			Priority:  0, // 所有任務優先級相同
		}
		o.tasks = append(o.tasks, task)
	}
}

// extractTableNamesFromKnowledge 從知識結果中提取表格名稱
func (o *TableAnalysisOrchestrator) extractTableNamesFromKnowledge(results []vectorstore.KnowledgeResult) []string {
	tableNames := make([]string, 0)
	seen := make(map[string]bool)

	for _, result := range results {
		content := result.Content

		// 嘗試解析為 JSON
		var knowledgeData map[string]interface{}
		if err := json.Unmarshal([]byte(content), &knowledgeData); err != nil {
			log.Printf("Failed to parse knowledge content as JSON: %v, trying text parsing", err)
			// 後備：文本解析
			o.extractTableNamesFromText(content, &tableNames, seen)
			continue
		}

		// 從 JSON 中提取表格名稱
		if tablesData, ok := knowledgeData["tables"].(map[string]interface{}); ok {
			for tableName := range tablesData {
				if tableName != "" && !seen[tableName] && o.isValidTableName(tableName) {
					log.Printf("Found valid table name from JSON: %s", tableName)
					tableNames = append(tableNames, tableName)
					seen[tableName] = true
				}
			}
		} else {
			// 如果沒有 tables 鍵，嘗試文本解析
			o.extractTableNamesFromText(content, &tableNames, seen)
		}
	}

	log.Printf("Extracted %d valid table names: %v", len(tableNames), tableNames)
	return tableNames
}

// extractTableNamesFromText 從文本中提取表格名稱（後備方法）
func (o *TableAnalysisOrchestrator) extractTableNamesFromText(content string, tableNames *[]string, seen map[string]bool) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 查找包含表格名稱的行
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "table") || strings.Contains(line, "表格") {
			// 跳過包含約束信息的行
			if strings.Contains(lowerLine, "constraint") || strings.Contains(lowerLine, "foreign") ||
				strings.Contains(lowerLine, "primary") || strings.Contains(lowerLine, "unique") {
				continue
			}

			// 嘗試提取表格名稱 - 查找單詞邊界
			words := strings.Fields(line)
			for _, word := range words {
				word = strings.Trim(word, ".,;:\"'")
				if o.isValidTableName(word) && !seen[word] {
					log.Printf("Found potential table name from text: %s", word)
					*tableNames = append(*tableNames, word)
					seen[word] = true
					break // 只取每行的第一個有效名稱
				}
			}
		}
	}
}

// isValidTableName 驗證表格名稱是否有效
func (o *TableAnalysisOrchestrator) isValidTableName(name string) bool {
	// 表格名稱應該只包含字母、數字和下劃線
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return false
		}
	}
	return len(name) > 0 && len(name) <= 50
}

// GetNextTask 獲取下一個待處理的任務
func (o *TableAnalysisOrchestrator) GetNextTask() *TableAnalysisTask {
	for _, task := range o.tasks {
		if task.Status == "pending" {
			return task
		}
	}
	return nil
}

// StartTask 開始處理任務
func (o *TableAnalysisOrchestrator) StartTask(task *TableAnalysisTask) {
	task.Status = "in_progress"
	o.currentTask = task
	log.Printf("Starting analysis for table: %s", task.TableName)
}

// CompleteTask 完成任務
func (o *TableAnalysisOrchestrator) CompleteTask(task *TableAnalysisTask, result *LLMAnalysisResult) {
	task.Status = "completed"
	task.Result = result
	o.results[task.TableName] = result
	o.currentTask = nil
	log.Printf("Completed analysis for table: %s", task.TableName)
}

// FailTask 任務失敗
func (o *TableAnalysisOrchestrator) FailTask(task *TableAnalysisTask, err error) {
	task.Status = "failed"
	task.Error = err
	o.currentTask = nil
	log.Printf("Failed analysis for table: %s, error: %v", task.TableName, err)
}

// AnalyzeTable 分析單個表格
func (o *TableAnalysisOrchestrator) AnalyzeTable(ctx context.Context, task *TableAnalysisTask) (*LLMAnalysisResult, error) {
	log.Printf("Analyzing table: %s", task.TableName)

	// 從向量存儲檢索表格相關知識
	query := fmt.Sprintf("table %s schema columns constraints sample data analysis", task.TableName)
	results, err := o.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, 5)
	if err != nil {
		log.Printf("Warning: Failed to retrieve table knowledge from vector store: %v", err)
		log.Printf("Falling back to file-based reader")
		// 後備方案：使用文件讀取器
		return o.analyzeTableWithFileReader(ctx, task)
	}

	if len(results) == 0 {
		log.Printf("No knowledge found for table %s in vector store, falling back to file reader", task.TableName)
		return o.analyzeTableWithFileReader(ctx, task)
	}

	// 從檢索到的知識構建表格摘要
	summary := o.buildTableSummaryFromKnowledge(task.TableName, results)

	// 準備 LLM 分析提示
	prompt := o.buildAnalysisPrompt(summary)

	// 調用 LLM 進行分析
	llmResponse, err := o.llmClient.AnalyzeTable(ctx, task.TableName, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze table with LLM: %v", err)
	}

	// 解析 LLM 回應
	result := &LLMAnalysisResult{
		TableName: task.TableName,
		Analysis:  llmResponse.Analysis,
		Timestamp: time.Now(),
	}

	return result, nil
}

// analyzeTableWithFileReader 使用文件讀取器分析表格（後備方案）
func (o *TableAnalysisOrchestrator) analyzeTableWithFileReader(ctx context.Context, task *TableAnalysisTask) (*LLMAnalysisResult, error) {
	// 獲取表格摘要
	summary, err := o.reader.GetTableSummary(task.TableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table summary: %v", err)
	}

	// 準備 LLM 分析提示
	prompt := o.buildAnalysisPrompt(summary)

	// 調用 LLM 進行分析
	llmResponse, err := o.llmClient.AnalyzeTable(ctx, task.TableName, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze table with LLM: %v", err)
	}

	// 解析 LLM 回應
	result := &LLMAnalysisResult{
		TableName: task.TableName,
		Analysis:  llmResponse.Analysis,
		Timestamp: time.Now(),
	}

	return result, nil
}

// buildTableSummaryFromKnowledge 從知識結果構建表格摘要
func (o *TableAnalysisOrchestrator) buildTableSummaryFromKnowledge(tableName string, results []vectorstore.KnowledgeResult) map[string]interface{} {
	summary := map[string]interface{}{
		"table_name":   tableName,
		"column_count": 0,
		"sample_count": 0,
		"columns":      []map[string]interface{}{},
		"constraints":  map[string]interface{}{},
		"samples":      []map[string]interface{}{},
	}

	// 合併所有相關知識內容
	var combinedContent strings.Builder
	for _, result := range results {
		combinedContent.WriteString(result.Content)
		combinedContent.WriteString("\n")
	}

	content := combinedContent.String()

	// 簡單的文本解析來提取信息
	// 這裡可以實現更複雜的解析邏輯
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "column_count") || strings.Contains(line, "欄位數量") {
			// 提取欄位數量
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				if count := o.extractNumber(parts[1]); count > 0 {
					summary["column_count"] = count
				}
			}
		} else if strings.Contains(line, "sample_count") || strings.Contains(line, "樣本數據數量") {
			// 提取樣本數量
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				if count := o.extractNumber(parts[1]); count > 0 {
					summary["sample_count"] = count
				}
			}
		}
	}

	return summary
}

// extractNumber 從字符串中提取數字
func (o *TableAnalysisOrchestrator) extractNumber(s string) int {
	s = strings.TrimSpace(s)
	for i, char := range s {
		if char < '0' || char > '9' {
			if i > 0 {
				if num, err := strconv.Atoi(s[:i]); err == nil {
					return num
				}
			}
			break
		}
	}
	if num, err := strconv.Atoi(s); err == nil {
		return num
	}
	return 0
}

// buildAnalysisPrompt 構建分析提示
func (o *TableAnalysisOrchestrator) buildAnalysisPrompt(summary map[string]interface{}) string {
	var prompt strings.Builder

	prompt.WriteString("請分析以下資料庫表格的商業邏輯和用途：\n\n")

	// 表格基本信息
	prompt.WriteString(fmt.Sprintf("表格名稱: %s\n", summary["table_name"]))
	prompt.WriteString(fmt.Sprintf("欄位數量: %d\n", summary["column_count"]))
	prompt.WriteString(fmt.Sprintf("樣本數據數量: %d\n", summary["sample_count"]))

	// 欄位信息
	if columns, ok := summary["columns"].([]map[string]interface{}); ok {
		prompt.WriteString("\n欄位結構:\n")
		for _, col := range columns {
			nullable := "NOT NULL"
			if col["nullable"].(bool) {
				nullable = "NULL"
			}
			prompt.WriteString(fmt.Sprintf("- %s: %s (%s)", col["name"], col["type"], nullable))
			if def, ok := col["default"]; ok && def != nil {
				prompt.WriteString(fmt.Sprintf(" DEFAULT %v", def))
			}
			prompt.WriteString("\n")
		}
	}

	// 約束信息
	if constraints, ok := summary["constraints"].(map[string]interface{}); ok {
		prompt.WriteString("\n約束:\n")
		if pks, ok := constraints["primary_keys"]; ok {
			if pkList, ok := pks.([]interface{}); ok && len(pkList) > 0 {
				prompt.WriteString(fmt.Sprintf("- 主鍵: %v\n", pkList))
			}
		}
		if fkCount, ok := constraints["foreign_keys_count"]; ok {
			prompt.WriteString(fmt.Sprintf("- 外鍵數量: %d\n", fkCount))
		}
		if ukCount, ok := constraints["unique_keys_count"]; ok {
			prompt.WriteString(fmt.Sprintf("- 唯一鍵數量: %d\n", ukCount))
		}
	}

	// 樣本數據
	if samples, ok := summary["samples"].([]map[string]interface{}); ok && len(samples) > 0 {
		prompt.WriteString("\n樣本數據:\n")
		for i, sample := range samples {
			if i >= 3 { // 只顯示前3個樣本
				break
			}
			prompt.WriteString(fmt.Sprintf("樣本 %d:\n", i+1))
			for key, value := range sample {
				prompt.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
			}
			prompt.WriteString("\n")
		}
	}

	prompt.WriteString("\n請基於以上資訊，描述這個表格的商業邏輯用途：\n")
	prompt.WriteString("1. 這個表格在整個系統中的角色和功能是什麼？\n")
	prompt.WriteString("2. 根據欄位定義和樣本數據，這個表格存儲的是什麼類型的業務數據？\n")
	prompt.WriteString("3. 這個表格與其他表格的業務關係是什麼？\n")
	prompt.WriteString("4. 從樣本數據可以看出什麼業務模式或用戶行為？\n")
	prompt.WriteString("5. 這個表格支持哪些業務流程？\n")
	prompt.WriteString("6. 根據約束和索引設計，可以推斷出這個表格的主要查詢場景是什麼？\n")

	prompt.WriteString("\n請用自然、易懂的語言描述這個表格的商業用途，不要過度關注技術細節。")

	return prompt.String()
}

// GetProgress 獲取分析進度
func (o *TableAnalysisOrchestrator) GetProgress() map[string]interface{} {
	total := len(o.tasks)
	completed := 0
	failed := 0
	inProgress := 0

	for _, task := range o.tasks {
		switch task.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		case "in_progress":
			inProgress++
		}
	}

	return map[string]interface{}{
		"total":       total,
		"completed":   completed,
		"failed":      failed,
		"in_progress": inProgress,
		"pending":     total - completed - failed - inProgress,
		"percentage":  float64(completed) / float64(total) * 100,
	}
}

// GetResults 獲取所有分析結果
func (o *TableAnalysisOrchestrator) GetResults() map[string]*LLMAnalysisResult {
	return o.results
}

// LLMClient LLM 客戶端
type LLMClient struct {
	config *config.Config
	client *http.Client
}

// LLMResponse LLM 回應
type LLMResponse struct {
	Analysis string `json:"analysis"`
}

// NewLLMClient 創建 LLM 客戶端
func NewLLMClient(cfg *config.Config) *LLMClient {
	return &LLMClient{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.LLM.TimeoutSeconds) * time.Second,
		},
	}
}

// AnalyzeTable 使用 LLM 分析表格
func (c *LLMClient) AnalyzeTable(ctx context.Context, tableName, prompt string) (*LLMResponse, error) {
	log.Printf("Sending analysis request to LLM for table: %s", tableName)

	// 構建請求
	requestBody := map[string]interface{}{
		"model": c.config.LLM.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一個資料庫及數據分析專家。請分析給定的表格結構，提供專業的見解和建議。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	// 發送請求到 LLM
	response, err := c.sendRequest(ctx, requestBody)
	if err != nil {
		log.Printf("LLM request failed, using fallback: %v", err)
		return c.fallbackResponse(tableName)
	}

	// 解析回應
	return c.parseResponse(response)
}

// sendRequest 發送請求到 LLM
func (c *LLMClient) sendRequest(ctx context.Context, requestBody map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 構建請求 URL
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", c.config.LLM.Host, c.config.LLM.Port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.LLM.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.LLM.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return response, nil
}

// parseResponse 解析 LLM 回應
func (c *LLMClient) parseResponse(response map[string]interface{}) (*LLMResponse, error) {
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("invalid response format: no choices")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: invalid choice")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: no message")
	}

	content, ok := message["content"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid response format: no content")
	}

	// 解析內容（這裡可以實現更複雜的解析邏輯）
	return c.parseContent(content)
}

// parseContent 解析 LLM 回應內容
func (c *LLMClient) parseContent(content string) (*LLMResponse, error) {
	// 簡單的解析邏輯 - 在實際實現中可以更複雜
	response := &LLMResponse{
		Analysis: content,
	}

	return response, nil
}

// fallbackResponse 後備回應（當 LLM 不可用時使用）
func (c *LLMClient) fallbackResponse(tableName string) (*LLMResponse, error) {
	log.Printf("Using fallback response for table: %s", tableName)

	return &LLMResponse{
		Analysis: fmt.Sprintf("表格 %s 的結構分析（後備模式）", tableName),
	}, nil
}

// CallLLM 通用 LLM 調用方法
func (c *LLMClient) CallLLM(prompt string) (string, error) {
	ctx := context.Background()
	log.Printf("Sending general LLM request")

	// 構建請求
	requestBody := map[string]interface{}{
		"model": c.config.LLM.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	// 發送請求到 LLM
	response, err := c.sendRequest(ctx, requestBody)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %v", err)
	}

	// 解析回應
	llmResponse, err := c.parseResponse(response)
	if err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %v", err)
	}

	return llmResponse.Analysis, nil
}

// MCPServer MCP 服務器接口
type MCPServer interface {
	// 這裡定義 MCP 服務器接口
	// 實際實現會在 mcp 包中
}
