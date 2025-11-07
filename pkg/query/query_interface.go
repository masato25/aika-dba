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

	for i, result := range results {
		ctx.WriteString(fmt.Sprintf("知識塊 %d:\n%s\n\n", i+1, result.Content))
	}

	return ctx.String()
}

// buildPrompt 構建 LLM 提示
func (qi *QueryInterface) buildPrompt(question, context string) string {
	template := "Based on the following database knowledge, answer the user's question.\n\n" +
		"%s\n\n" +
		"Question: %s\n\n" +
		"If you need to get actual data from the database, provide an SQL query statement (enclosed in ```sql).\n" +
		"Otherwise, answer directly. If you need more information, ask the user.\n\n" +
		"Response format:\n" +
		"- If SQL needed: Explain what data you need, then provide the SQL\n" +
		"- If not needed: Answer directly"
	return fmt.Sprintf(template, context, question)
}

// Query 處理自然語言查詢
func (qi *QueryInterface) Query(question string) (string, error) {
	qi.logger.Printf("處理查詢: %s", question)

	// 檢索相關知識
	results, err := qi.km.RetrievePhaseKnowledge("database_knowledge", question, 5)
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
