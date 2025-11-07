package query

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// QueryInterface 負責處理自然語言查詢
type QueryInterface struct {
	km     *vectorstore.KnowledgeManager
	llm    *llm.Client
	db     *sql.DB
	logger *log.Logger
}

// NewQueryInterface 創建新的查詢接口
func NewQueryInterface(km *vectorstore.KnowledgeManager, llmClient *llm.Client, db *sql.DB, logger *log.Logger) *QueryInterface {
	return &QueryInterface{
		km:     km,
		llm:    llmClient,
		db:     db,
		logger: logger,
	}
}

// buildContext 構建上下文
func (qi *QueryInterface) buildContext(results []vectorstore.KnowledgeResult) string {
	var ctx strings.Builder
	ctx.WriteString("資料庫知識:\n")

	// 限制上下文長度，避免超過 LLM 限制
	maxLength := 2000 // 限制上下文總長度
	currentLength := 0

	for i, result := range results {
		content := result.Content
		if currentLength+len(content) > maxLength {
			// 如果添加這個內容會超過限制，截斷它
			remaining := maxLength - currentLength
			if remaining > 100 { // 至少保留 100 字符
				content = content[:remaining-3] + "..."
			} else {
				break
			}
		}

		ctx.WriteString(fmt.Sprintf("知識塊 %d:\n%s\n\n", i+1, content))
		currentLength += len(content)

		if currentLength >= maxLength {
			break
		}
	}

	return ctx.String()
}

// buildPrompt 構建 LLM 提示
func (qi *QueryInterface) buildPrompt(question, context string) string {
	template := "基於以下資料庫知識回答用戶問題:\n\n%s\n\n問題: %s\n\n" +
		"如果需要從資料庫獲取實際數據，請提供 SQL 查詢語句（用 ```sql 包圍）。\n" +
		"否則直接回答。如果需要更多資訊，請詢問用戶。\n\n" +
		"回應格式:\n" +
		"- 需要 SQL 時: 解釋需要什麼數據，然後提供 SQL\n" +
		"- 不需要時: 直接回答"
	return fmt.Sprintf(template, context, question)
}

// Query 處理自然語言查詢
func (qi *QueryInterface) Query(question string) (string, error) {
	qi.logger.Printf("處理查詢: %s", question)

	// 檢索相關知識
	results, err := qi.km.RetrievePhaseKnowledge("database_knowledge", question, 3)
	if err != nil {
		return "", fmt.Errorf("檢索知識失敗: %v", err)
	}

	if len(results) == 0 {
		return "抱歉，知識庫中沒有相關資訊可以回答這個問題。", nil
	}

	// 構建上下文
	contextStr := qi.buildContext(results)

	// 構建提示
	prompt := qi.buildPrompt(question, contextStr)

	// 調用 LLM 生成回答
	llmResponse, err := qi.llm.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		return "", fmt.Errorf("LLM 生成回答失敗: %v", err)
	}

	// 檢查是否包含 SQL
	sqlQuery := qi.extractSQL(llmResponse)
	if sqlQuery != "" {
		// 執行 SQL 並獲取結果
		sqlResult, err := qi.executeSQL(sqlQuery)
		if err != nil {
			return "", fmt.Errorf("SQL 執行失敗: %v", err)
		}

		// 格式化最終回答
		finalAnswer := fmt.Sprintf("%s\n\nSQL Command:\n```sql\n%s\n```\n\nSQL Output:\n%s", llmResponse, sqlQuery, sqlResult)
		return finalAnswer, nil
	}

	qi.logger.Println("查詢處理完成")
	return llmResponse, nil
}

// extractSQL 從 LLM 回應中提取 SQL 查詢
func (qi *QueryInterface) extractSQL(response string) string {
	// 查找 ```sql ... ``` 包圍的內容
	re := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// executeSQL 執行 SQL 查詢並格式化結果
func (qi *QueryInterface) executeSQL(query string) (string, error) {
	rows, err := qi.db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(strings.Join(columns, "\t") + "\n")
	result.WriteString(strings.Repeat("-", len(strings.Join(columns, "\t"))*2) + "\n")

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return "", err
		}

		var row []string
		for _, val := range values {
			if val == nil {
				row = append(row, "NULL")
			} else {
				switch v := val.(type) {
				case []byte:
					// 將 byte slice 轉換為 string
					row = append(row, string(v))
				default:
					row = append(row, fmt.Sprintf("%v", val))
				}
			}
		}
		result.WriteString(strings.Join(row, "\t") + "\n")
		rowCount++
		if rowCount > 100 { // 限制輸出行數
			result.WriteString("... (truncated)\n")
			break
		}
	}

	if rowCount == 0 {
		result.WriteString("(no rows)\n")
	}

	return result.String(), nil
}
