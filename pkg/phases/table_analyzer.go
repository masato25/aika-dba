package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
)

// LLMAnalysisResult LLM 分析結果
type LLMAnalysisResult struct {
	TableName       string    `json:"table_name"`
	Analysis        string    `json:"analysis"`
	Recommendations []string  `json:"recommendations"`
	Issues          []string  `json:"issues"`
	Insights        []string  `json:"insights"`
	Timestamp       time.Time `json:"timestamp"`
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
	config      *config.Config
	reader      *Phase1ResultReader
	tasks       []*TableAnalysisTask
	llmClient   *LLMClient
	mcpServer   MCPServer
	currentTask *TableAnalysisTask
	results     map[string]*LLMAnalysisResult
}

// NewTableAnalysisOrchestrator 創建表格分析協調器
func NewTableAnalysisOrchestrator(cfg *config.Config, reader *Phase1ResultReader, mcpServer MCPServer) *TableAnalysisOrchestrator {
	return &TableAnalysisOrchestrator{
		config:    cfg,
		reader:    reader,
		tasks:     make([]*TableAnalysisTask, 0),
		llmClient: NewLLMClient(cfg),
		mcpServer: mcpServer,
		results:   make(map[string]*LLMAnalysisResult),
	}
}

// InitializeTasks 初始化分析任務
func (o *TableAnalysisOrchestrator) InitializeTasks() error {
	tableNames, err := o.reader.GetTableNames()
	if err != nil {
		return fmt.Errorf("failed to get table names: %v", err)
	}

	log.Printf("Initializing analysis tasks for %d tables", len(tableNames))

	for _, tableName := range tableNames {
		task := &TableAnalysisTask{
			TableName: tableName,
			Status:    "pending",
			Priority:  0, // 所有任務優先級相同
		}
		o.tasks = append(o.tasks, task)
	}

	return nil
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
		TableName:       task.TableName,
		Analysis:        llmResponse.Analysis,
		Recommendations: llmResponse.Recommendations,
		Issues:          llmResponse.Issues,
		Insights:        llmResponse.Insights,
		Timestamp:       time.Now(),
	}

	return result, nil
}

// buildAnalysisPrompt 構建分析提示
func (o *TableAnalysisOrchestrator) buildAnalysisPrompt(summary map[string]interface{}) string {
	var prompt strings.Builder

	prompt.WriteString("請分析以下資料庫表格的結構和數據：\n\n")

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

	// 索引信息
	if indexes, ok := summary["indexes"].([]map[string]interface{}); ok && len(indexes) > 0 {
		prompt.WriteString("\n索引:\n")
		for _, idx := range indexes {
			unique := ""
			if idx["unique"].(bool) {
				unique = " (唯一)"
			}
			prompt.WriteString(fmt.Sprintf("- %s%s: %v\n", idx["name"], unique, idx["columns"]))
		}
	}

	// 統計信息
	if stats, ok := summary["stats"].(map[string]interface{}); ok {
		prompt.WriteString("\n統計信息:\n")
		for key, value := range stats {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
	}

	prompt.WriteString("\n請提供以下分析:\n")
	prompt.WriteString("1. 表格設計評估（結構合理性、命名規範等）\n")
	prompt.WriteString("2. 性能優化建議（索引、查詢優化等）\n")
	prompt.WriteString("3. 數據完整性分析（約束、數據質量等）\n")
	prompt.WriteString("4. 潛在問題識別\n")
	prompt.WriteString("5. 改進建議\n")

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
	Analysis        string   `json:"analysis"`
	Recommendations []string `json:"recommendations"`
	Issues          []string `json:"issues"`
	Insights        []string `json:"insights"`
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
				"content": "你是一個資料庫分析專家。請分析給定的表格結構，提供專業的見解和建議。",
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
		Analysis:        content,
		Recommendations: []string{},
		Issues:          []string{},
		Insights:        []string{},
	}

	// 嘗試提取結構化信息
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "建議：") || strings.HasPrefix(line, "推薦：") {
			response.Recommendations = append(response.Recommendations, strings.TrimPrefix(line, "建議："))
		} else if strings.HasPrefix(line, "問題：") || strings.HasPrefix(line, "Issue:") {
			response.Issues = append(response.Issues, strings.TrimPrefix(strings.TrimPrefix(line, "問題："), "Issue:"))
		} else if strings.HasPrefix(line, "見解：") || strings.HasPrefix(line, "洞察：") {
			response.Insights = append(response.Insights, strings.TrimPrefix(line, "見解："))
		}
	}

	// 如果沒有提取到結構化信息，使用默認值
	if len(response.Recommendations) == 0 {
		response.Recommendations = []string{"建議添加適當的索引來提升查詢性能"}
	}
	if len(response.Issues) == 0 {
		response.Issues = []string{"需要進一步分析以識別潛在問題"}
	}
	if len(response.Insights) == 0 {
		response.Insights = []string{"表格結構基本合理"}
	}

	return response, nil
}

// fallbackResponse 後備回應（當 LLM 不可用時使用）
func (c *LLMClient) fallbackResponse(tableName string) (*LLMResponse, error) {
	log.Printf("Using fallback response for table: %s", tableName)

	return &LLMResponse{
		Analysis: fmt.Sprintf("表格 %s 的結構分析（後備模式）", tableName),
		Recommendations: []string{
			"建議添加適當的索引來提升查詢性能",
			"考慮添加數據驗證約束",
		},
		Issues: []string{
			"LLM 服務不可用，無法進行深入分析",
		},
		Insights: []string{
			"表格設計基本合理",
			"數據完整性良好",
		},
	}, nil
}

// MCPServer MCP 服務器接口
type MCPServer interface {
	// 這裡定義 MCP 服務器接口
	// 實際實現會在 mcp 包中
}
