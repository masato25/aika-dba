package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// Phase1PostRunner Phase 1 後置處理執行器 - 互動式數據庫分析和清理
type Phase1PostRunner struct {
	config       *config.Config
	llmClient    *llm.Client
	knowledgeMgr *vectorstore.KnowledgeManager
}

// NewPhase1PostRunner 創建 Phase 1 後置處理執行器
func NewPhase1PostRunner(cfg *config.Config) (*Phase1PostRunner, error) {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		return nil, err
	}

	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)

	return &Phase1PostRunner{
		config:       cfg,
		llmClient:    llmClient,
		knowledgeMgr: knowledgeMgr,
	}, nil
}

// Run 執行 Phase 1 後置處理
func (p *Phase1PostRunner) Run() error {
	log.Println("=== Starting Phase 1 Post-Processing: Interactive Database Analysis & Cleanup ===")

	// 讀取 Phase 1 的分析結果
	phase1Data, err := p.loadPhase1Results()
	if err != nil {
		return fmt.Errorf("failed to load phase1 results: %v", err)
	}

	// 檢查是否有用戶回答文件
	userResponses, err := p.loadUserResponses()
	hasResponses := err == nil && len(userResponses) > 0

	if hasResponses {
		// 如果有用戶回答，處理回答並生成決策
		log.Println("Found user responses, processing decisions...")
		
		// 讀取問題詳細信息
		questions, err := p.loadQuestions()
		if err != nil {
			log.Printf("Warning: Failed to load questions: %v", err)
		}
		
		decisions := p.processUserResponses(phase1Data, userResponses, questions)
		
		// 生成最終分析結果
		finalAnalysis := p.generateFinalAnalysis(phase1Data, decisions)
		
		// 創建輸出
		output := map[string]interface{}{
			"database":                p.config.Database.DBName,
			"timestamp":               time.Now(),
			"phase":                   "phase1_post",
			"user_responses":          userResponses,
			"questions":               questions,
			"decisions":               decisions,
			"final_analysis":          finalAnalysis,
		}

		// 寫入文件
		if err := p.writeOutput(output, "knowledge/phase1_post_analysis.json"); err != nil {
			return err
		}

		// 將知識存儲到向量數據庫
		if err := p.knowledgeMgr.StorePhaseKnowledge("phase1_post", output); err != nil {
			log.Printf("Warning: Failed to store phase1_post knowledge in vector store: %v", err)
		} else {
			log.Printf("Phase 1 post-processing knowledge stored in vector database")
		}

		log.Println("Phase 1 post-processing completed with user decisions applied")
		
	} else {
		// 如果沒有用戶回答，生成問題讓用戶回答
		log.Println("No user responses found, generating questions for review...")
		questions := p.generateQuestions(phase1Data)
		
		// 顯示問題
		p.displayQuestions(questions)
		
		// 保存問題供用戶回答
		if err := p.saveQuestionsForUser(questions); err != nil {
			return fmt.Errorf("failed to save questions: %v", err)
		}
		
		log.Println("Questions generated and saved. Please review and provide answers in phase1_post_responses.json")
	}

	return nil
}

// loadUserResponses 讀取用戶回答
func (p *Phase1PostRunner) loadUserResponses() (map[string]interface{}, error) {
	file, err := os.Open("knowledge/phase1_post_responses.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// generateQuestions 使用 LLM 生成問題
func (p *Phase1PostRunner) generateQuestions(phase1Data map[string]interface{}) []map[string]interface{} {
	// 創建數據摘要而不是使用完整數據
	summary := p.createDataSummary(phase1Data)
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	summaryStr := string(summaryJSON)

	// 構建提示詞
	prompt := fmt.Sprintf(`請分析以下數據庫分析摘要，並提出具體的問題來幫助用戶決定如何處理這些表格。

數據庫摘要：
%s

請提出以下類型的問題：
1. 對於看起來未使用的表格：這個表格是否還在被應用程序使用？
2. 對於空的表格：是否可以安全刪除？
3. 對於低使用量的表格：是否仍然需要？

請以 JSON 格式返回問題列表，每個問題包含：
- question_id: 問題的唯一標識符
- question_type: "usage_check", "empty_table_check", "low_usage_check"
- question: 具體問題內容
- related_tables: 相關的表格名稱列表
- options: 可選的回答選項

只返回 JSON 格式，不要其他解釋。`, summaryStr)

	// 調用 LLM
	response, err := p.llmClient.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		log.Printf("Warning: Failed to generate questions with LLM: %v", err)
		// 返回默認問題
		questions := p.generateDefaultQuestions(phase1Data)
		log.Printf("Generated %d default questions", len(questions))
		return questions
	}

	// 解析 LLM 回應
	var questions []map[string]interface{}
	if err := json.Unmarshal([]byte(response), &questions); err != nil {
		log.Printf("Warning: Failed to parse LLM response as array, trying as object: %v", err)
		// 嘗試解析為對象
		var responseObj map[string]interface{}
		if err := json.Unmarshal([]byte(response), &responseObj); err != nil {
			log.Printf("Warning: Failed to parse LLM response: %v", err)
			questions = p.generateDefaultQuestions(phase1Data)
		} else {
			// 如果是對象，嘗試提取 questions 字段
			if q, ok := responseObj["questions"].([]interface{}); ok {
				for _, item := range q {
					if question, ok := item.(map[string]interface{}); ok {
						questions = append(questions, question)
					}
				}
			}
		}
	}

	// 如果沒有生成問題，也返回默認問題
	if len(questions) == 0 {
		log.Printf("Warning: No questions generated by LLM, using defaults")
		questions = p.generateDefaultQuestions(phase1Data)
	}

	log.Printf("Generated %d questions", len(questions))
	return questions
}

// generateDefaultQuestions 生成默認問題（當 LLM 失敗時）
func (p *Phase1PostRunner) generateDefaultQuestions(phase1Data map[string]interface{}) []map[string]interface{} {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var questions []map[string]interface{}
	questionID := 1

	// 檢查空表
	for tableName, tableData := range tables {
		tableInfo := tableData.(map[string]interface{})
		stats, ok := tableInfo["stats"].(map[string]interface{})
		if !ok {
			continue
		}
		
		recordCount, ok := getRowCount(stats)
		if !ok {
			continue
		}
		
		if recordCount == 0 {
			questions = append(questions, map[string]interface{}{
				"question_id":    fmt.Sprintf("q%d", questionID),
				"question_type":  "empty_table_check",
				"question":       fmt.Sprintf("表格 '%s' 是空的（0條記錄）。是否可以安全刪除這個表格？", tableName),
				"related_tables": []string{tableName},
				"options":        []string{"可以刪除", "保留（仍在使用）", "需要進一步檢查"},
			})
			questionID++
		}
	}

	// 檢查低使用量表格
	for tableName, tableData := range tables {
		tableInfo := tableData.(map[string]interface{})
		stats, ok := tableInfo["stats"].(map[string]interface{})
		if !ok {
			continue
		}
		
		recordCount, ok := getRowCount(stats)
		if !ok || recordCount == 0 {
			continue
		}
		
		if recordCount > 0 && recordCount < 10 {
			questions = append(questions, map[string]interface{}{
				"question_id":    fmt.Sprintf("q%d", questionID),
				"question_type":  "usage_check",
				"question":       fmt.Sprintf("表格 '%s' 只有 %d 條記錄。這是否表示這個表格已經不再使用？", tableName, recordCount),
				"related_tables": []string{tableName},
				"options":        []string{"不再使用，可以刪除", "仍在使用", "測試數據，可以清理"},
			})
			questionID++
		}
	}

	return questions
}

// displayQuestions 顯示問題給用戶
func (p *Phase1PostRunner) displayQuestions(questions []map[string]interface{}) {
	fmt.Println("\n=== Phase 1 Post-Processing Questions ===")
	fmt.Println("請回答以下問題來幫助確定數據庫清理策略：")
	fmt.Println()

	for i, q := range questions {
		fmt.Printf("%d. [%s] %s\n", i+1, q["question_type"], q["question"])
		
		if options, ok := q["options"].([]interface{}); ok {
			fmt.Println("   選項：")
			for j, option := range options {
				fmt.Printf("   %d) %s\n", j+1, option)
			}
		}
		
		if tables, ok := q["related_tables"].([]interface{}); ok {
			fmt.Printf("   相關表格: %v\n", tables)
		}
		fmt.Println()
	}

	fmt.Println("請將您的回答保存到 knowledge/phase1_post_responses.json 文件中")
	fmt.Println("格式示例：")
	fmt.Println(`{
  "q1": "可以刪除",
  "q2": "仍在使用",
  ...
}`)
}

// saveQuestionsForUser 保存問題供用戶回答
func (p *Phase1PostRunner) saveQuestionsForUser(questions []map[string]interface{}) error {
	data := map[string]interface{}{
		"generated_at": time.Now(),
		"questions":    questions,
		"instructions": "請回答以下問題。對於每個問題，請提供您的決定。",
	}

	return p.writeOutput(data, "knowledge/phase1_post_questions.json")
}

// processUserResponses 處理用戶回答
func (p *Phase1PostRunner) processUserResponses(phase1Data map[string]interface{}, userResponses map[string]interface{}, questionsData map[string]interface{}) map[string]interface{} {
	// 從問題數據中提取問題列表
	questions := []map[string]interface{}{}
	if q, ok := questionsData["questions"].([]interface{}); ok {
		for _, item := range q {
			if question, ok := item.(map[string]interface{}); ok {
				questions = append(questions, question)
			}
		}
	}

	// 創建問題 ID 到表格名稱的映射
	questionToTables := make(map[string][]string)
	for _, q := range questions {
		if qid, ok := q["question_id"].(string); ok {
			if tables, ok := q["related_tables"].([]interface{}); ok {
				tableNames := []string{}
				for _, t := range tables {
					if tableName, ok := t.(string); ok {
						tableNames = append(tableNames, tableName)
					}
				}
				questionToTables[qid] = tableNames
			}
		}
	}

	decisions := map[string]interface{}{
		"table_decisions": map[string]interface{}{},
		"summary": map[string]interface{}{
			"tables_to_drop": []string{},
			"tables_to_keep": []string{},
			"tables_to_merge": []map[string]interface{}{},
			"tables_to_review": []string{},
		},
	}

	// 處理每個用戶回答
	for questionID, response := range userResponses {
		if questionID == "timestamp" || questionID == "notes" {
			continue // 跳過元數據
		}

		responseStr := fmt.Sprintf("%v", response)
		tables := questionToTables[questionID]
		
		// 根據回答決定行動
		switch {
		case containsString(responseStr, "刪除") || containsString(responseStr, "drop") || containsString(responseStr, "可以刪除"):
			decisions["summary"].(map[string]interface{})["tables_to_drop"] = append(
				decisions["summary"].(map[string]interface{})["tables_to_drop"].([]string), 
				tables...)
		case containsString(responseStr, "保留") || containsString(responseStr, "keep") || containsString(responseStr, "仍在使用"):
			decisions["summary"].(map[string]interface{})["tables_to_keep"] = append(
				decisions["summary"].(map[string]interface{})["tables_to_keep"].([]string), 
				tables...)
		case containsString(responseStr, "合併") || containsString(responseStr, "merge"):
			for _, table := range tables {
				decisions["summary"].(map[string]interface{})["tables_to_merge"] = append(
					decisions["summary"].(map[string]interface{})["tables_to_merge"].([]map[string]interface{}), 
					map[string]interface{}{"table": table, "decision": responseStr, "question_id": questionID})
			}
		default:
			decisions["summary"].(map[string]interface{})["tables_to_review"] = append(
				decisions["summary"].(map[string]interface{})["tables_to_review"].([]string), 
				tables...)
		}
	}

	return decisions
}

// generateFinalAnalysis 生成最終分析結果
func (p *Phase1PostRunner) generateFinalAnalysis(phase1Data map[string]interface{}, decisions map[string]interface{}) map[string]interface{} {
	// 基於用戶決策生成最終分析
	// 排除已決定刪除或合併的表格
	
	summary := decisions["summary"].(map[string]interface{})
	
	return map[string]interface{}{
		"total_questions_answered": len(decisions["table_decisions"].(map[string]interface{})),
		"tables_to_drop_count": len(summary["tables_to_drop"].([]string)),
		"tables_to_keep_count": len(summary["tables_to_keep"].([]string)),
		"tables_to_merge_count": len(summary["tables_to_merge"].([]map[string]interface{})),
		"tables_to_review_count": len(summary["tables_to_review"].([]string)),
		"decisions_applied": decisions,
	}
}

// loadPhase1Results 讀取 Phase 1 的分析結果
func (p *Phase1PostRunner) loadPhase1Results() (map[string]interface{}, error) {
	file, err := os.Open("knowledge/phase1_analysis.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// loadQuestions 讀取問題文件
func (p *Phase1PostRunner) loadQuestions() (map[string]interface{}, error) {
	questionsFile := "knowledge/phase1_post_questions.json"
	
	data, err := os.ReadFile(questionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read questions file: %w", err)
	}
	
	var questionsData map[string]interface{}
	if err := json.Unmarshal(data, &questionsData); err != nil {
		return nil, fmt.Errorf("failed to parse questions file: %w", err)
	}
	
	return questionsData, nil
}

// createDataSummary 創建數據摘要
func (p *Phase1PostRunner) createDataSummary(phase1Data map[string]interface{}) map[string]interface{} {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "no tables data found"}
	}

	summary := map[string]interface{}{
		"database": phase1Data["database"],
		"total_tables": len(tables),
		"empty_tables": []string{},
		"low_usage_tables": []map[string]interface{}{},
		"high_usage_tables": []map[string]interface{}{},
	}

	for tableName, tableData := range tables {
		tableInfo := tableData.(map[string]interface{})
		
		// 從 stats 中獲取 row_count
		stats, ok := tableInfo["stats"].(map[string]interface{})
		if !ok {
			continue
		}
		
		recordCount, ok := getRowCount(stats)
		if !ok {
			continue
		}

		if recordCount == 0 {
			summary["empty_tables"] = append(summary["empty_tables"].([]string), tableName)
		} else if recordCount < 10 {
			summary["low_usage_tables"] = append(summary["low_usage_tables"].([]map[string]interface{}), 
				map[string]interface{}{"name": tableName, "count": recordCount})
		} else if recordCount > 10000 {
			summary["high_usage_tables"] = append(summary["high_usage_tables"].([]map[string]interface{}), 
				map[string]interface{}{"name": tableName, "count": recordCount})
		}
	}

	return summary
}

// writeOutput 寫入輸出到文件
func (p *Phase1PostRunner) writeOutput(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	log.Printf("Phase 1 post-processing completed. Results saved to %s", filename)
	return nil
}

// getRowCount 從 stats 中安全獲取 row_count
func getRowCount(stats map[string]interface{}) (int, bool) {
	if rowCountNum, ok := stats["row_count"].(float64); ok {
		return int(rowCountNum), true
	}
	if rowCountStr, ok := stats["row_count"].(string); ok {
		if count, err := strconv.Atoi(rowCountStr); err == nil {
			return count, true
		}
	}
	return 0, false
}

// containsString 檢查字符串是否包含子字符串（不區分大小寫）
func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}