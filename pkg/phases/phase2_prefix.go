package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// Phase2PrefixRunner Phase 2 前置處理執行器 - 欄位深度分析
type Phase2PrefixRunner struct {
	config       *config.Config
	llmClient    *llm.Client
	knowledgeMgr *vectorstore.KnowledgeManager
}

// NewPhase2PrefixRunner 創建 Phase 2 前置處理執行器
func NewPhase2PrefixRunner(cfg *config.Config) (*Phase2PrefixRunner, error) {
	log.Println("DEBUG: Creating knowledge manager...")
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge manager: %v", err)
	}
	log.Println("DEBUG: Knowledge manager created successfully")

	log.Println("DEBUG: Creating LLM client...")
	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)
	log.Println("DEBUG: LLM client created successfully")

	return &Phase2PrefixRunner{
		config:       cfg,
		llmClient:    llmClient,
		knowledgeMgr: knowledgeMgr,
	}, nil
}

// Run 執行 Phase 2 前置處理
func (p *Phase2PrefixRunner) Run() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== Starting Phase 2 Prefix: Column Deep Analysis ===")

	// 讀取 Phase 1 的分析結果
	log.Println("Loading Phase 1 analysis results...")
	phase1Data, err := p.loadPhase1Results()
	if err != nil {
		return fmt.Errorf("failed to load phase1 results: %v", err)
	}
	log.Printf("Phase 1 data loaded successfully. Database: %v", phase1Data["database"])

	// 檢查是否有用戶回答文件
	log.Println("Checking for user responses...")
	userResponses, err := p.loadUserResponses()
	hasResponses := err == nil && len(userResponses) > 0

	if hasResponses {
		log.Printf("Found user responses (%d answers), processing decisions...", len(userResponses))

		// 讀取問題詳細信息
		log.Println("Loading questions data...")
		questions, err := p.loadQuestions()
		if err != nil {
			log.Printf("Warning: Failed to load questions: %v", err)
		}

		log.Println("Processing user responses and generating decisions...")
		decisions := p.processUserResponses(phase1Data, userResponses, questions)

		log.Println("Generating final analysis...")
		finalAnalysis := p.generateFinalAnalysis(phase1Data, decisions)

		// 創建輸出
		log.Println("Creating output data structure...")
		output := map[string]interface{}{
			"database":       p.config.Database.DBName,
			"timestamp":      time.Now(),
			"phase":          "phase2_prefix",
			"user_responses": userResponses,
			"questions":      questions,
			"decisions":      decisions,
			"final_analysis": finalAnalysis,
		}

		// 寫入文件
		log.Println("Writing analysis results to file...")
		if err := p.writeOutput(output, "knowledge/phase2_prefix_analysis.json"); err != nil {
			return err
		}

		// 將知識存儲到向量數據庫
		log.Println("Storing knowledge in vector database...")
		if err := p.knowledgeMgr.StorePhaseKnowledge("phase2_prefix", output); err != nil {
			log.Printf("Warning: Failed to store phase2_prefix knowledge in vector store: %v", err)
		} else {
			log.Printf("Phase 2 prefix knowledge stored in vector database")
		}

		log.Println("Phase 2 prefix analysis completed with user decisions applied")

	} else {
		log.Println("No user responses found, generating questions for review...")

		log.Println("Starting question generation process...")
		questions := p.generateQuestions(phase1Data)

		log.Printf("Generated %d questions total", len(questions))

		// 顯示問題
		log.Println("Displaying questions for user review...")
		p.displayQuestions(questions)

		// 保存問題供用戶回答
		log.Println("Saving questions to file...")
		if err := p.saveQuestionsForUser(questions); err != nil {
			return fmt.Errorf("failed to save questions: %v", err)
		}

		log.Println("Questions generated and saved. Please review and provide answers in phase2_prefix_responses.json")
	}

	return nil
}

// loadUserResponses 讀取用戶回答
func (p *Phase2PrefixRunner) loadUserResponses() (map[string]interface{}, error) {
	file, err := os.Open("knowledge/phase2_prefix_responses.json")
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
func (p *Phase2PrefixRunner) generateQuestions(phase1Data map[string]interface{}) []map[string]interface{} {
	log.Println("Creating column analysis summary for LLM...")
	// 創建數據摘要而不是使用完整數據
	summary := p.createColumnAnalysisSummary(phase1Data)
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	summaryStr := string(summaryJSON)

	log.Printf("Column analysis summary created. Total tables: %d, columns with issues: %d",
		len(summary["column_analysis"].([]map[string]interface{})), len(summary["column_analysis"].([]map[string]interface{})))

	// 構建提示詞
	log.Println("Preparing LLM prompt for question generation...")
	prompt := fmt.Sprintf(`請分析以下數據庫欄位分析摘要，並提出具體的問題來幫助用戶決定如何處理這些欄位。

數據庫欄位摘要：
%s

請分析每個表格的欄位，提出以下類型的問題：
1. 對於看起來沒有使用的欄位（大量空值）：這個欄位是否還在被使用？
2. 對於可能需要搜集 collections 的欄位（開發者懶惰沒有建 table 直接寫 string）：這個欄位應該轉換為關聯表格嗎？
3. 對於使用 enum 的欄位：這個欄位的枚舉值是否完整定義？
4. 對於使用 int 但沒有定義的欄位：這個欄位是否需要更好的定義？

請以 JSON 格式返回問題列表，每個問題包含：
- question_id: 問題的唯一標識符
- question_type: "unused_column_check", "collection_check", "enum_check", "int_definition_check"
- question: 具體問題內容
- table_name: 相關的表格名稱
- column_name: 相關的欄位名稱
- options: 可選的回答選項
- analysis_data: 相關的分析數據（用於 LLM 自動回答）

只返回 JSON 格式，不要其他解釋。`, summaryStr)

	// 調用 LLM
	log.Println("Calling LLM to generate questions...")
	response, err := p.llmClient.GenerateCompletion(context.Background(), prompt)
	if err != nil {
		log.Printf("Warning: Failed to generate questions with LLM: %v", err)
		log.Println("Falling back to default question generation...")
		// 返回默認問題
		questions := p.generateDefaultQuestions(phase1Data)
		log.Printf("Generated %d default questions", len(questions))
		return questions
	}

	log.Println("LLM response received, parsing questions...")

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

	log.Printf("Successfully generated %d questions", len(questions))
	return questions
}

// generateDefaultQuestions 生成默認問題（當 LLM 失敗時）
func (p *Phase2PrefixRunner) generateDefaultQuestions(phase1Data map[string]interface{}) []map[string]interface{} {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		log.Println("No tables data found in phase1 results")
		return []map[string]interface{}{}
	}

	log.Printf("Starting default question generation for %d tables...", len(tables))

	var questions []map[string]interface{}
	questionID := 1
	totalTables := len(tables)
	processedTables := 0

	// 分析每個表格的欄位
	for tableName, tableData := range tables {
		processedTables++
		log.Printf("Processing table %d/%d: %s", processedTables, totalTables, tableName)

		tableInfo := tableData.(map[string]interface{})

		// 獲取 schema
		schema, ok := tableInfo["schema"].([]interface{})
		if !ok {
			log.Printf("  No schema found for table %s, skipping", tableName)
			continue
		}

		log.Printf("  Analyzing %d columns in table %s", len(schema), tableName)

		// 分析每個欄位
		columnsProcessed := 0
		for _, colInfo := range schema {
			col, ok := colInfo.(map[string]interface{})
			if !ok {
				continue
			}

			colName, ok := col["name"].(string)
			if !ok {
				continue
			}

			colType, ok := col["type"].(string)
			if !ok {
				continue
			}

			columnsProcessed++

			// 每處理 10 個欄位輸出一次進度
			if columnsProcessed%10 == 0 {
				log.Printf("    Processed %d/%d columns in table %s", columnsProcessed, len(schema), tableName)
			}

			// 檢查可能的問題類型
			if p.isPotentiallyUnusedColumn(col, tableInfo) {
				questions = append(questions, map[string]interface{}{
					"question_id":   fmt.Sprintf("q%d", questionID),
					"question_type": "unused_column_check",
					"question":      fmt.Sprintf("表格 '%s' 的欄位 '%s' 看起來可能沒有在使用（大量空值）。這個欄位是否還在被應用程序使用？", tableName, colName),
					"table_name":    tableName,
					"column_name":   colName,
					"options":       []string{"不再使用，可以移除", "仍在使用", "需要進一步檢查"},
					"analysis_data": map[string]interface{}{
						"column_type": colType,
						"nullable":    col["nullable"],
					},
				})
				questionID++
			}

			if p.isPotentialCollectionColumn(col, tableInfo) {
				questions = append(questions, map[string]interface{}{
					"question_id":   fmt.Sprintf("q%d", questionID),
					"question_type": "collection_check",
					"question":      fmt.Sprintf("表格 '%s' 的欄位 '%s' 看起來像是儲存了多個值的集合。這個欄位是否應該轉換為關聯表格？", tableName, colName),
					"table_name":    tableName,
					"column_name":   colName,
					"options":       []string{"應該轉換為關聯表格", "保持現狀", "需要進一步檢查"},
					"analysis_data": map[string]interface{}{
						"column_type": colType,
						"nullable":    col["nullable"],
					},
				})
				questionID++
			}

			if p.isEnumColumn(col, tableInfo) {
				questions = append(questions, map[string]interface{}{
					"question_id":   fmt.Sprintf("q%d", questionID),
					"question_type": "enum_check",
					"question":      fmt.Sprintf("表格 '%s' 的欄位 '%s' 看起來使用了枚舉值。這個欄位的枚舉值是否完整定義？", tableName, colName),
					"table_name":    tableName,
					"column_name":   colName,
					"options":       []string{"枚舉值完整", "枚舉值不完整，需要補充", "需要進一步檢查"},
					"analysis_data": map[string]interface{}{
						"column_type": colType,
						"nullable":    col["nullable"],
					},
				})
				questionID++
			}

			if p.isUndefinedIntColumn(col, tableInfo) {
				questions = append(questions, map[string]interface{}{
					"question_id":   fmt.Sprintf("q%d", questionID),
					"question_type": "int_definition_check",
					"question":      fmt.Sprintf("表格 '%s' 的欄位 '%s' 使用了 int 類型但看起來沒有明確的定義。這個欄位是否需要更好的定義？", tableName, colName),
					"table_name":    tableName,
					"column_name":   colName,
					"options":       []string{"需要更好的定義", "定義已經足夠", "需要進一步檢查"},
					"analysis_data": map[string]interface{}{
						"column_type": colType,
						"nullable":    col["nullable"],
					},
				})
				questionID++
			}

			if p.isValueCollectionColumn(col, tableInfo) {
				questions = append(questions, map[string]interface{}{
					"question_id":   fmt.Sprintf("q%d", questionID),
					"question_type": "value_collection_check",
					"question":      fmt.Sprintf("表格 '%s' 的欄位 '%s' 包含了多個不同的值。這個欄位是否需要搜集可用的值選項？", tableName, colName),
					"table_name":    tableName,
					"column_name":   colName,
					"options":       []string{"需要搜集值選項", "不需要，值動態生成", "需要進一步檢查"},
					"analysis_data": map[string]interface{}{
						"column_type": colType,
						"nullable":    col["nullable"],
					},
				})
				questionID++
			}
		}

		log.Printf("  Completed analysis for table %s, questions so far: %d", tableName, len(questions))
	}

	log.Printf("Default question generation completed. Total questions: %d", len(questions))
	return questions
}
func (p *Phase2PrefixRunner) isPotentiallyUnusedColumn(col map[string]interface{}, tableInfo map[string]interface{}) bool {
	// 檢查是否可為空且在樣本數據中大量為空
	nullable, ok := col["nullable"].(bool)
	if !ok || !nullable {
		return false
	}

	// 檢查樣本數據
	samples, ok := tableInfo["samples"].([]interface{})
	if !ok || len(samples) == 0 {
		return false
	}

	// 計算空值比例
	colName := col["name"].(string)
	nullCount := 0
	totalCount := len(samples)

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[colName]
		if !exists || value == nil || value == "" {
			nullCount++
		}
	}

	// 如果空值比例超過 80%，認為可能沒有使用
	return float64(nullCount)/float64(totalCount) > 0.8
}

// isPotentialCollectionColumn 檢查欄位是否可能是集合類型
func (p *Phase2PrefixRunner) isPotentialCollectionColumn(col map[string]interface{}, tableInfo map[string]interface{}) bool {
	colType, ok := col["type"].(string)
	if !ok {
		return false
	}

	// 如果是 ARRAY 類型，直接視為集合欄位
	if strings.ToUpper(colType) == "ARRAY" {
		return true
	}

	// 檢查是否是可能儲存集合的類型
	if !strings.Contains(strings.ToLower(colType), "text") && !strings.Contains(strings.ToLower(colType), "varchar") {
		return false
	}

	// 檢查樣本數據中是否有集合特徵
	samples, ok := tableInfo["samples"].([]interface{})
	if !ok || len(samples) == 0 {
		return false
	}

	colName := col["name"].(string)
	collectionIndicators := 0

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[colName]
		if !exists {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)

		// 檢查集合指標
		if strings.Contains(valueStr, ",") || strings.Contains(valueStr, ";") ||
			strings.Contains(valueStr, "[") || strings.Contains(valueStr, "{") ||
			strings.Contains(valueStr, "|") {
			collectionIndicators++
		}
	}

	// 如果有多個樣本顯示集合特徵
	return collectionIndicators > 2
}

// isValueCollectionColumn 檢查欄位是否需要搜集值
func (p *Phase2PrefixRunner) isValueCollectionColumn(col map[string]interface{}, tableInfo map[string]interface{}) bool {
	colType, ok := col["type"].(string)
	if !ok {
		return false
	}

	colName, ok := col["name"].(string)
	if !ok {
		return false
	}

	// 檢查樣本數據
	samples, ok := tableInfo["samples"].([]interface{})
	if !ok || len(samples) < 3 {
		return false
	}

	// 計算非空值的數量和唯一值
	nonNullCount := 0
	uniqueValues := make(map[string]bool)

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[colName]
		if exists && value != nil && value != "" {
			valueStr := fmt.Sprintf("%v", value)
			uniqueValues[valueStr] = true
			nonNullCount++
		}
	}

	// 如果非空值太少，跳過
	if nonNullCount < 3 {
		return false
	}

	uniqueCount := len(uniqueValues)

	// 對於狀態類欄位（包含 status, type, state 等關鍵字）
	if strings.Contains(strings.ToLower(colName), "status") ||
		strings.Contains(strings.ToLower(colName), "type") ||
		strings.Contains(strings.ToLower(colName), "state") ||
		strings.Contains(strings.ToLower(colName), "gender") ||
		strings.Contains(strings.ToLower(colName), "category") {
		return uniqueCount <= 20 // 狀態類欄位通常枚舉值不會太多
	}

	// 對於名稱類欄位（包含 name, title 等關鍵字）
	if strings.Contains(strings.ToLower(colName), "name") ||
		strings.Contains(strings.ToLower(colName), "title") ||
		strings.Contains(strings.ToLower(colName), "city") ||
		strings.Contains(strings.ToLower(colName), "address") {
		return uniqueCount > 1 && uniqueCount <= len(samples)/2 // 名稱類欄位重複度可能較高
	}

	// 對於 ID 類欄位
	if strings.Contains(strings.ToLower(colName), "id") &&
		!strings.Contains(strings.ToLower(colName), "uuid") &&
		!strings.Contains(strings.ToLower(colName), "guid") {
		return uniqueCount > 1 && uniqueCount <= len(samples)/3 // ID 類欄位重複度不高
	}

	// 對於 varchar/char 類型，如果唯一值比例適中，可能需要搜集
	if strings.Contains(strings.ToLower(colType), "varchar") ||
		strings.Contains(strings.ToLower(colType), "char") {
		uniqueRatio := float64(uniqueCount) / float64(nonNullCount)
		return uniqueRatio > 0.1 && uniqueRatio < 0.9 && uniqueCount <= 50
	}

	return false
}

// isEnumColumn 檢查欄位是否使用枚舉
func (p *Phase2PrefixRunner) isEnumColumn(col map[string]interface{}, tableInfo map[string]interface{}) bool {
	colType, ok := col["type"].(string)
	if !ok {
		return false
	}

	// 檢查是否是可能的枚舉類型
	if !strings.Contains(strings.ToLower(colType), "varchar") && !strings.Contains(strings.ToLower(colType), "char") {
		return false
	}

	// 檢查樣本數據中的唯一值數量
	samples, ok := tableInfo["samples"].([]interface{})
	if !ok || len(samples) < 5 {
		return false
	}

	colName := col["name"].(string)
	uniqueValues := make(map[string]bool)

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[colName]
		if exists && value != nil && value != "" {
			uniqueValues[fmt.Sprintf("%v", value)] = true
		}
	}

	// 如果唯一值數量少於總樣本數的 20%，可能是枚舉
	uniqueCount := len(uniqueValues)
	return uniqueCount > 1 && uniqueCount < len(samples)/5
}

// isUndefinedIntColumn 檢查 int 欄位是否沒有定義
func (p *Phase2PrefixRunner) isUndefinedIntColumn(col map[string]interface{}, tableInfo map[string]interface{}) bool {
	colType, ok := col["type"].(string)
	if !ok {
		return false
	}

	// 檢查是否是 int 類型
	if !strings.Contains(strings.ToLower(colType), "int") {
		return false
	}

	// 檢查是否有約束或外鍵
	constraints, ok := tableInfo["constraints"].(map[string]interface{})
	if !ok {
		return true // 沒有約束，可能需要定義
	}

	// 檢查外鍵
	if foreignKeys, ok := constraints["foreign_keys"].([]interface{}); ok && len(foreignKeys) > 0 {
		return false // 有外鍵，已經定義
	}

	// 檢查主鍵
	if primaryKeys, ok := constraints["primary_keys"].([]interface{}); ok {
		colName := col["name"].(string)
		for _, pk := range primaryKeys {
			if pkStr, ok := pk.(string); ok && pkStr == colName {
				return false // 是主鍵，已經定義
			}
		}
	}

	return true // 沒有外鍵也不是主鍵，可能需要更好的定義
}

// displayQuestions 顯示問題給用戶
func (p *Phase2PrefixRunner) displayQuestions(questions []map[string]interface{}) {
	fmt.Println("\n=== Phase 2 Prefix Questions ===")
	fmt.Println("請回答以下問題來幫助確定欄位優化策略：")
	fmt.Println()

	for i, q := range questions {
		fmt.Printf("%d. [%s] %s\n", i+1, q["question_type"], q["question"])

		if table, ok := q["table_name"].(string); ok {
			if col, ok := q["column_name"].(string); ok {
				fmt.Printf("   表格: %s, 欄位: %s\n", table, col)
			}
		}

		if options, ok := q["options"].([]interface{}); ok {
			fmt.Println("   選項：")
			for j, option := range options {
				fmt.Printf("   %d) %s\n", j+1, option)
			}
		}
		fmt.Println()
	}

	fmt.Println("請將您的回答保存到 knowledge/phase2_prefix_responses.json 文件中")
	fmt.Println("格式示例：")
	fmt.Println(`{
  "q1": "不再使用，可以移除",
  "q2": "應該轉換為關聯表格",
  ...
}`)
}

// saveQuestionsForUser 保存問題供用戶回答
func (p *Phase2PrefixRunner) saveQuestionsForUser(questions []map[string]interface{}) error {
	data := map[string]interface{}{
		"generated_at": time.Now(),
		"questions":    questions,
		"instructions": "請回答以下問題。對於每個問題，請提供您的決定。",
	}

	return p.writeOutput(data, "knowledge/phase2_prefix_questions.json")
}

// processUserResponses 處理用戶回答
func (p *Phase2PrefixRunner) processUserResponses(phase1Data map[string]interface{}, userResponses map[string]interface{}, questionsData map[string]interface{}) map[string]interface{} {
	log.Println("Processing user responses...")

	// 從問題數據中提取問題列表
	questions := []map[string]interface{}{}
	if q, ok := questionsData["questions"].([]interface{}); ok {
		for _, item := range q {
			if question, ok := item.(map[string]interface{}); ok {
				questions = append(questions, question)
			}
		}
	}

	log.Printf("Loaded %d questions for processing", len(questions))

	decisions := map[string]interface{}{
		"column_decisions": map[string]interface{}{},
		"summary": map[string]interface{}{
			"columns_to_remove":  []map[string]interface{}{},
			"columns_to_convert": []map[string]interface{}{},
			"columns_to_define":  []map[string]interface{}{},
			"columns_to_review":  []map[string]interface{}{},
			"enum_values_found":  map[string]interface{}{},
			"collection_values":  map[string]interface{}{},
		},
	}

	// 處理每個用戶回答
	log.Printf("Processing %d user responses...", len(userResponses))
	processedResponses := 0

	for questionID, response := range userResponses {
		if questionID == "timestamp" || questionID == "notes" {
			continue // 跳過元數據
		}

		processedResponses++
		if processedResponses%10 == 0 {
			log.Printf("Processed %d/%d responses...", processedResponses, len(userResponses))
		}

		responseStr := fmt.Sprintf("%v", response)

		// 找到對應的問題
		var currentQuestion map[string]interface{}
		for _, q := range questions {
			if qid, ok := q["question_id"].(string); ok && qid == questionID {
				currentQuestion = q
				break
			}
		}

		if currentQuestion == nil {
			log.Printf("Warning: No matching question found for response ID: %s", questionID)
			continue
		}

		tableName, _ := currentQuestion["table_name"].(string)
		columnName, _ := currentQuestion["column_name"].(string)
		questionType, _ := currentQuestion["question_type"].(string)

		columnInfo := map[string]interface{}{
			"table":         tableName,
			"column":        columnName,
			"question_id":   questionID,
			"decision":      responseStr,
			"question_type": questionType,
		}

		// 根據問題類型和回答決定行動
		switch questionType {
		case "unused_column_check":
			if containsString(responseStr, "不再使用") || containsString(responseStr, "可以移除") {
				decisions["summary"].(map[string]interface{})["columns_to_remove"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_remove"].([]map[string]interface{}),
					columnInfo)
			} else {
				decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
					columnInfo)
			}

		case "collection_check":
			if containsString(responseStr, "應該轉換") {
				decisions["summary"].(map[string]interface{})["columns_to_convert"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_convert"].([]map[string]interface{}),
					columnInfo)
				// 自動收集集合值
				log.Printf("Collecting collection values for %s.%s", tableName, columnName)
				p.collectCollectionValues(phase1Data, tableName, columnName, decisions)
			} else {
				decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
					columnInfo)
			}

		case "enum_check":
			if containsString(responseStr, "枚舉值完整") {
				// 自動收集枚舉值
				log.Printf("Collecting enum values for %s.%s", tableName, columnName)
				p.collectEnumValues(phase1Data, tableName, columnName, decisions)
			} else if containsString(responseStr, "枚舉值不完整") {
				decisions["summary"].(map[string]interface{})["columns_to_define"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_define"].([]map[string]interface{}),
					columnInfo)
				// 仍然收集現有的枚舉值
				log.Printf("Collecting enum values for %s.%s (incomplete)", tableName, columnName)
				p.collectEnumValues(phase1Data, tableName, columnName, decisions)
			} else {
				decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
					columnInfo)
			}

		case "int_definition_check":
			if containsString(responseStr, "需要更好的定義") {
				decisions["summary"].(map[string]interface{})["columns_to_define"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_define"].([]map[string]interface{}),
					columnInfo)
			} else {
				decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
					columnInfo)
			}

		case "value_collection_check":
			if containsString(responseStr, "需要搜集值選項") {
				decisions["summary"].(map[string]interface{})["columns_to_define"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_define"].([]map[string]interface{}),
					columnInfo)
				// 自動收集值選項
				log.Printf("Collecting value options for %s.%s", tableName, columnName)
				p.collectValueOptions(phase1Data, tableName, columnName, decisions)
			} else {
				decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
					decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
					columnInfo)
			}

		default:
			decisions["summary"].(map[string]interface{})["columns_to_review"] = append(
				decisions["summary"].(map[string]interface{})["columns_to_review"].([]map[string]interface{}),
				columnInfo)
		}
	}

	log.Printf("User response processing completed. Total decisions made: %d", processedResponses)
	return decisions
}

// collectEnumValues 收集枚舉值
func (p *Phase2PrefixRunner) collectEnumValues(phase1Data map[string]interface{}, tableName, columnName string, decisions map[string]interface{}) {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		return
	}

	tableData, ok := tables[tableName].(map[string]interface{})
	if !ok {
		return
	}

	samples, ok := tableData["samples"].([]interface{})
	if !ok {
		return
	}

	uniqueValues := make(map[string]int)
	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[columnName]
		if exists && value != nil && value != "" {
			valueStr := fmt.Sprintf("%v", value)
			uniqueValues[valueStr]++
		}
	}

	key := fmt.Sprintf("%s.%s", tableName, columnName)
	decisions["summary"].(map[string]interface{})["enum_values_found"].(map[string]interface{})[key] = uniqueValues
}

// collectValueOptions 收集欄位的值選項
func (p *Phase2PrefixRunner) collectValueOptions(phase1Data map[string]interface{}, tableName, columnName string, decisions map[string]interface{}) {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		return
	}

	tableData, ok := tables[tableName].(map[string]interface{})
	if !ok {
		return
	}

	samples, ok := tableData["samples"].([]interface{})
	if !ok {
		return
	}

	uniqueValues := make(map[string]int)
	var examples []string

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[columnName]
		if exists && value != nil && value != "" {
			valueStr := fmt.Sprintf("%v", value)
			uniqueValues[valueStr]++

			// 保存一些例子
			if len(examples) < 10 && !containsStringInSlice(examples, valueStr) {
				examples = append(examples, valueStr)
			}
		}
	}

	key := fmt.Sprintf("%s.%s", tableName, columnName)
	if decisions["summary"].(map[string]interface{})["enum_values_found"] == nil {
		decisions["summary"].(map[string]interface{})["enum_values_found"] = map[string]interface{}{}
	}
	decisions["summary"].(map[string]interface{})["enum_values_found"].(map[string]interface{})[key] = map[string]interface{}{
		"unique_values": uniqueValues,
		"examples":      examples,
		"total_unique":  len(uniqueValues),
	}
}

// collectCollectionValues 收集集合值
func (p *Phase2PrefixRunner) collectCollectionValues(phase1Data map[string]interface{}, tableName, columnName string, decisions map[string]interface{}) {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		return
	}

	tableData, ok := tables[tableName].(map[string]interface{})
	if !ok {
		return
	}

	samples, ok := tableData["samples"].([]interface{})
	if !ok {
		return
	}

	var collectionExamples []string
	uniqueItems := make(map[string]int)

	for _, sample := range samples {
		sampleData, ok := sample.(map[string]interface{})
		if !ok {
			continue
		}

		value, exists := sampleData[columnName]
		if exists && value != nil && value != "" {
			valueStr := fmt.Sprintf("%v", value)

			// 嘗試解析集合
			var items []string
			if strings.Contains(valueStr, ",") {
				items = strings.Split(valueStr, ",")
			} else if strings.Contains(valueStr, ";") {
				items = strings.Split(valueStr, ";")
			} else if strings.Contains(valueStr, "|") {
				items = strings.Split(valueStr, "|")
			} else {
				items = []string{valueStr}
			}

			// 收集唯一項目
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item != "" {
					uniqueItems[item]++
				}
			}

			// 保存一些例子
			if len(collectionExamples) < 5 {
				collectionExamples = append(collectionExamples, valueStr)
			}
		}
	}

	key := fmt.Sprintf("%s.%s", tableName, columnName)
	decisions["summary"].(map[string]interface{})["collection_values"].(map[string]interface{})[key] = map[string]interface{}{
		"unique_items": uniqueItems,
		"examples":     collectionExamples,
	}
}

// generateFinalAnalysis 生成最終分析結果
func (p *Phase2PrefixRunner) generateFinalAnalysis(phase1Data map[string]interface{}, decisions map[string]interface{}) map[string]interface{} {
	summary := decisions["summary"].(map[string]interface{})

	return map[string]interface{}{
		"total_questions_answered": len(decisions["column_decisions"].(map[string]interface{})),
		"columns_to_remove_count":  len(summary["columns_to_remove"].([]map[string]interface{})),
		"columns_to_convert_count": len(summary["columns_to_convert"].([]map[string]interface{})),
		"columns_to_define_count":  len(summary["columns_to_define"].([]map[string]interface{})),
		"columns_to_review_count":  len(summary["columns_to_review"].([]map[string]interface{})),
		"enum_columns_found":       len(summary["enum_values_found"].(map[string]interface{})),
		"collection_columns_found": len(summary["collection_values"].(map[string]interface{})),
		"decisions_applied":        decisions,
	}
}

// loadPhase1Results 讀取 Phase 1 的分析結果
func (p *Phase2PrefixRunner) loadPhase1Results() (map[string]interface{}, error) {
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
func (p *Phase2PrefixRunner) loadQuestions() (map[string]interface{}, error) {
	questionsFile := "knowledge/phase2_prefix_questions.json"

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

// createColumnAnalysisSummary 創建欄位分析摘要
func (p *Phase2PrefixRunner) createColumnAnalysisSummary(phase1Data map[string]interface{}) map[string]interface{} {
	tables, ok := phase1Data["tables"].(map[string]interface{})
	if !ok {
		log.Println("No tables data found for column analysis summary")
		return map[string]interface{}{"error": "no tables data found"}
	}

	log.Printf("Creating column analysis summary for %d tables...", len(tables))

	summary := map[string]interface{}{
		"database":        phase1Data["database"],
		"total_tables":    len(tables),
		"column_analysis": []map[string]interface{}{},
	}

	totalTables := len(tables)
	processedTables := 0
	totalColumnsAnalyzed := 0

	for tableName, tableData := range tables {
		processedTables++
		log.Printf("Analyzing table %d/%d: %s", processedTables, totalTables, tableName)

		tableInfo := tableData.(map[string]interface{})

		// 獲取 schema
		schema, ok := tableInfo["schema"].([]interface{})
		if !ok {
			log.Printf("  No schema found for table %s", tableName)
			continue
		}

		log.Printf("  Processing %d columns in table %s", len(schema), tableName)

		// 分析每個欄位
		columnsWithIssues := 0
		columnsProcessed := 0
		for _, colInfo := range schema {
			col, ok := colInfo.(map[string]interface{})
			if !ok {
				continue
			}

			colName, ok := col["name"].(string)
			if !ok {
				continue
			}

			colType, ok := col["type"].(string)
			if !ok {
				continue
			}

			totalColumnsAnalyzed++
			columnsProcessed++

			// 每處理 20 個欄位輸出一次進度
			if columnsProcessed%20 == 0 {
				log.Printf("    Processed %d/%d columns in table %s", columnsProcessed, len(schema), tableName)
			}

			// 創建欄位分析
			columnAnalysis := map[string]interface{}{
				"table_name":  tableName,
				"column_name": colName,
				"column_type": colType,
				"nullable":    col["nullable"],
				"issues":      []string{},
			}

			// 檢查可能的問題
			if p.isPotentiallyUnusedColumn(col, tableInfo) {
				columnAnalysis["issues"] = append(columnAnalysis["issues"].([]string), "potentially_unused")
			}

			if p.isPotentialCollectionColumn(col, tableInfo) {
				columnAnalysis["issues"] = append(columnAnalysis["issues"].([]string), "potential_collection")
			}

			if p.isEnumColumn(col, tableInfo) {
				columnAnalysis["issues"] = append(columnAnalysis["issues"].([]string), "enum_usage")
			}

			if p.isValueCollectionColumn(col, tableInfo) {
				columnAnalysis["issues"] = append(columnAnalysis["issues"].([]string), "value_collection_needed")
			}

			if p.isUndefinedIntColumn(col, tableInfo) {
				columnAnalysis["issues"] = append(columnAnalysis["issues"].([]string), "undefined_int")
			}

			// 只添加有問題的欄位
			if len(columnAnalysis["issues"].([]string)) > 0 {
				summary["column_analysis"] = append(summary["column_analysis"].([]map[string]interface{}), columnAnalysis)
				columnsWithIssues++
			}
		}

		log.Printf("  Table %s completed: %d columns analyzed, %d with issues", tableName, len(schema), columnsWithIssues)
	}

	log.Printf("Column analysis summary completed. Total columns analyzed: %d, columns with issues: %d",
		totalColumnsAnalyzed, len(summary["column_analysis"].([]map[string]interface{})))

	return summary
}

// writeOutput 寫入輸出到文件
func (p *Phase2PrefixRunner) writeOutput(data interface{}, filename string) error {
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

	log.Printf("Phase 2 prefix analysis completed. Results saved to %s", filename)
	return nil
}

// containsStringInSlice 檢查字符串是否在切片中
func containsStringInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
