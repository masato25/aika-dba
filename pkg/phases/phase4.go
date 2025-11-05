package phases

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
	lua "github.com/yuin/gopher-lua"
)

// Dimension 維度定義
type Dimension struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // people, time, product, event, location
	Description string   `json:"description"`
	SourceTable string   `json:"source_table"`
	KeyFields   []string `json:"key_fields"`
	Attributes  []string `json:"attributes"`
	BusinessUse string   `json:"business_use"`
}

// FactTable 事實表定義
type FactTable struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SourceTable string   `json:"source_table"`
	Measures    []string `json:"measures"`
	Dimensions  []string `json:"dimensions"`
}

// Phase4Runner Phase 4 執行器 - 使用 Lua 規則引擎進行維度建模
type Phase4Runner struct {
	config       *config.Config
	db           *sql.DB
	knowledgeMgr *vectorstore.KnowledgeManager
	luaState     *lua.LState
}

// NewPhase4Runner 創建 Phase 4 執行器
func NewPhase4Runner(cfg *config.Config, db *sql.DB) *Phase4Runner {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Printf("Warning: Failed to create knowledge manager: %v", err)
		knowledgeMgr = nil
	}

	return &Phase4Runner{
		config:       cfg,
		db:           db,
		knowledgeMgr: knowledgeMgr,
	}
}

// Run 執行 Phase 4 維度建模分析（使用 Lua 規則引擎）
func (p *Phase4Runner) Run() error {
	log.Println("=== Starting Phase 4: Lua Rule Engine Dimension Modeling ===")

	// 從向量存儲檢索 Phase 3 維度規則
	phase3Rules, err := p.retrievePhase3Rules()
	if err != nil {
		return fmt.Errorf("failed to retrieve Phase 3 rules: %v", err)
	}

	// 從向量存儲檢索 Phase 2 分析結果
	phase2Results, err := p.retrievePhase2Knowledge()
	if err != nil {
		return fmt.Errorf("failed to retrieve Phase 2 knowledge: %v", err)
	}

	// 初始化 Lua VM 並載入規則
	if err := p.initLuaVM(phase3Rules); err != nil {
		return fmt.Errorf("failed to initialize Lua VM: %v", err)
	}
	defer p.luaState.Close()

	// 使用 Lua 規則引擎生成維度
	dimensions, factTables, err := p.executeLuaRules(phase2Results)
	if err != nil {
		return fmt.Errorf("failed to execute Lua rules: %v", err)
	}

	// 生成維度建模報告 - 按照分類組織
	report := p.generateCategorizedReport(dimensions, factTables)

	// 保存報告並存儲到向量數據庫
	if err := p.writeOutput(report, "knowledge/phase4_dimensions.json"); err != nil {
		return err
	}

	// 將 Phase 4 結果存儲到向量數據庫
	if err := p.storePhase4Results(dimensions, factTables); err != nil {
		log.Printf("Warning: Failed to store Phase 4 results in vector store: %v", err)
	}

	log.Println("Phase 4 completed successfully - dimension modeling analysis generated")
	return nil
}

// retrievePhase3Rules 從向量存儲檢索 Phase 3 維度規則
func (p *Phase4Runner) retrievePhase3Rules() (string, error) {
	if p.knowledgeMgr == nil {
		return "", fmt.Errorf("knowledge manager not available")
	}

	// 檢索 Phase 3 的維度規則
	query := "dimension rules lua script phase3"
	results, err := p.knowledgeMgr.RetrievePhaseKnowledge("phase3", query, 5)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve phase3 rules: %v", err)
	}

	if len(results) == 0 {
		log.Printf("No Phase 3 rules found in vector store, will use default file")
		return "", nil // 返回空字符串，將使用默認文件
	}

	// 從檢索到的知識中提取 Lua 規則
	for _, result := range results {
		content := result.Content

		// 檢查是否包含 Lua 腳本
		if strings.Contains(content, "function detect_dimensions") {
			return content, nil
		}
	}

	log.Printf("No valid Lua rules found in vector store, will use default file")
	return "", nil
}

// retrievePhase2Knowledge 從向量存儲檢索 Phase 2 知識
func (p *Phase4Runner) retrievePhase2Knowledge() (map[string]*LLMAnalysisResult, error) {
	if p.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	// 首先嘗試從向量存儲檢索
	query := "business logic analysis AI insights recommendations"
	results, err := p.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, 20)
	if err != nil {
		log.Printf("Failed to retrieve from vector store: %v, trying JSON file", err)
		return p.retrievePhase2FromJSON()
	}

	if len(results) == 0 {
		log.Printf("No Phase 2 knowledge found in vector store, trying JSON file")
		return p.retrievePhase2FromJSON()
	}

	// 從檢索到的知識中重建分析結果
	phase2Results := make(map[string]*LLMAnalysisResult)

	for _, result := range results {
		content := result.Content

		// 嘗試解析為 JSON
		var analysisData map[string]interface{}
		if err := json.Unmarshal([]byte(content), &analysisData); err != nil {
			log.Printf("Failed to parse phase2 knowledge as JSON: %v", err)
			continue
		}

		// 提取分析結果
		if analysisResults, ok := analysisData["analysis_results"].(map[string]interface{}); ok {
			for tableName, resultData := range analysisResults {
				if resultMap, ok := resultData.(map[string]interface{}); ok {
					llmResult := &LLMAnalysisResult{
						TableName: tableName,
					}

					if analysis, ok := resultMap["analysis"].(string); ok {
						llmResult.Analysis = analysis
					}

					if recommendations, ok := resultMap["recommendations"].([]interface{}); ok {
						llmResult.Recommendations = make([]string, len(recommendations))
						for i, rec := range recommendations {
							if recStr, ok := rec.(string); ok {
								llmResult.Recommendations[i] = recStr
							}
						}
					}

					if issues, ok := resultMap["issues"].([]interface{}); ok {
						llmResult.Issues = make([]string, len(issues))
						for i, issue := range issues {
							if issueStr, ok := issue.(string); ok {
								llmResult.Issues[i] = issueStr
							}
						}
					}

					if insights, ok := resultMap["insights"].([]interface{}); ok {
						llmResult.Insights = make([]string, len(insights))
						for i, insight := range insights {
							if insightStr, ok := insight.(string); ok {
								llmResult.Insights[i] = insightStr
							}
						}
					}

					if timestamp, ok := resultMap["timestamp"].(string); ok {
						if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
							llmResult.Timestamp = t
						} else {
							llmResult.Timestamp = time.Now()
						}
					} else {
						llmResult.Timestamp = time.Now()
					}

					phase2Results[tableName] = llmResult
				}
			}
		}
	}

	if len(phase2Results) == 0 {
		log.Printf("No valid Phase 2 results from vector store, trying JSON file")
		return p.retrievePhase2FromJSON()
	}

	log.Printf("Retrieved %d table analysis results from vector store", len(phase2Results))
	return phase2Results, nil
}

// retrievePhase2FromJSON 從 JSON 文件檢索 Phase 2 知識（備用方案）
func (p *Phase4Runner) retrievePhase2FromJSON() (map[string]*LLMAnalysisResult, error) {
	reader := NewPhase2ResultReader("knowledge/phase2_analysis.json")
	return reader.GetAnalysisResults()
}

// initLuaVM 初始化 Lua 虛擬機並載入規則
func (p *Phase4Runner) initLuaVM(rulesContent string) error {
	p.luaState = lua.NewState()

	// 如果提供了規則內容，直接執行，否則載入文件
	if rulesContent != "" {
		if err := p.luaState.DoString(rulesContent); err != nil {
			return fmt.Errorf("failed to execute Lua rules string: %v", err)
		}
	} else {
		if err := p.luaState.DoFile("knowledge/dimension_rules.lua"); err != nil {
			return fmt.Errorf("failed to load Lua rules file: %v", err)
		}
	}

	return nil
}

// retrieveTableAnalysisFromFile 從 phase1_analysis.json 文件中檢索表格分析信息
func (p *Phase4Runner) retrieveTableAnalysisFromFile(tableName string) (*TableAnalysisResult, error) {
	// 讀取 phase1_analysis.json 文件
	data, err := os.ReadFile("knowledge/phase1_analysis.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read phase1_analysis.json: %v", err)
	}

	var phase1Data struct {
		Tables map[string]interface{} `json:"tables"`
	}

	if err := json.Unmarshal(data, &phase1Data); err != nil {
		return nil, fmt.Errorf("failed to parse phase1_analysis.json: %v", err)
	}

	// 查找指定的表格
	tableData, ok := phase1Data.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in phase1_analysis.json", tableName)
	}

	tableMap, ok := tableData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid table data format for %s", tableName)
	}

	tableAnalysis := &TableAnalysisResult{}

	// 解析 schema
	if schema, ok := tableMap["schema"].([]interface{}); ok {
		tableAnalysis.Schema = make([]map[string]interface{}, len(schema))
		for i, col := range schema {
			if colMap, ok := col.(map[string]interface{}); ok {
				tableAnalysis.Schema[i] = colMap
			}
		}
	}

	// 解析約束
	if constraints, ok := tableMap["constraints"].(map[string]interface{}); ok {
		tableAnalysis.Constraints = constraints
	}

	// 解析索引
	if indexes, ok := tableMap["indexes"].([]interface{}); ok {
		tableAnalysis.Indexes = make([]map[string]interface{}, len(indexes))
		for i, idx := range indexes {
			if idxMap, ok := idx.(map[string]interface{}); ok {
				tableAnalysis.Indexes[i] = idxMap
			}
		}
	}

	// 解析樣本
	if samples, ok := tableMap["samples"].([]interface{}); ok {
		tableAnalysis.Samples = make([]map[string]interface{}, len(samples))
		for i, sample := range samples {
			if sampleMap, ok := sample.(map[string]interface{}); ok {
				tableAnalysis.Samples[i] = sampleMap
			}
		}
	}

	// 解析統計
	if stats, ok := tableMap["stats"].(map[string]interface{}); ok {
		tableAnalysis.Stats = stats
	}

	return tableAnalysis, nil
}

// executeLuaRules 執行 Lua 規則來生成維度和事實表
func (p *Phase4Runner) executeLuaRules(phase2Results map[string]*LLMAnalysisResult) ([]Dimension, []FactTable, error) {
	dimensions := []Dimension{}
	factTables := []FactTable{}

	// 跟踪已生成的維度名稱以避免重複
	existingDimensions := make(map[string]bool)

	// 獲取 Lua 函數
	detectDimensionsFn := p.luaState.GetGlobal("detect_dimensions")
	if detectDimensionsFn.Type() != lua.LTFunction {
		return nil, nil, fmt.Errorf("detect_dimensions function not found in Lua script")
	}

	detectFactTablesFn := p.luaState.GetGlobal("detect_fact_tables")
	if detectFactTablesFn.Type() != lua.LTFunction {
		return nil, nil, fmt.Errorf("detect_fact_tables function not found in Lua script")
	}

	// 為每個表格執行規則
	for tableName, result := range phase2Results {
		// 準備表格元數據
		tableMeta := p.createTableMeta(tableName, result)

		// 檢測維度
		if err := p.luaState.CallByParam(lua.P{
			Fn:      detectDimensionsFn,
			NRet:    1,
			Protect: true,
		}, lua.LString(tableName), tableMeta); err != nil {
			log.Printf("Warning: Failed to detect dimensions for table %s: %v", tableName, err)
			continue
		}

		// 處理維度結果
		dimsResult := p.luaState.Get(-1)
		p.luaState.Pop(1)

		if dimsTable, ok := dimsResult.(*lua.LTable); ok {
			tableDimensions := p.convertLuaTableToDimensions(dimsTable)
			// 只添加未重複的維度
			for _, dim := range tableDimensions {
				if !existingDimensions[dim.Name] {
					dimensions = append(dimensions, dim)
					existingDimensions[dim.Name] = true
				}
			}
		}

		// 檢測事實表
		if err := p.luaState.CallByParam(lua.P{
			Fn:      detectFactTablesFn,
			NRet:    1,
			Protect: true,
		}, lua.LString(tableName), tableMeta); err != nil {
			log.Printf("Warning: Failed to detect fact tables for table %s: %v", tableName, err)
			continue
		}

		// 處理事實表結果
		factsResult := p.luaState.Get(-1)
		p.luaState.Pop(1)

		if factsTable, ok := factsResult.(*lua.LTable); ok {
			tableFacts := p.convertLuaTableToFactTables(factsTable)
			factTables = append(factTables, tableFacts...)
		}
	}

	return dimensions, factTables, nil
}

// createTableMeta 創建表格元數據（Lua 表格）
func (p *Phase4Runner) createTableMeta(tableName string, result *LLMAnalysisResult) *lua.LTable {
	meta := p.luaState.NewTable()

	// 從 phase1_analysis.json 文件中讀取表格結構
	tableAnalysis, err := p.retrieveTableAnalysisFromFile(tableName)
	if err != nil {
		log.Printf("Warning: Failed to get table analysis for %s from file: %v", tableName, err)
		// 返回空的元數據
		meta.RawSetString("columns", p.luaState.NewTable())
		meta.RawSetString("existing_dimensions", p.luaState.NewTable())
		return meta
	}

	// 添加欄位列表
	columns := p.luaState.NewTable()
	for i, col := range tableAnalysis.Schema {
		colTable := p.luaState.NewTable()
		if name, ok := col["name"].(string); ok {
			colTable.RawSetString("name", lua.LString(name))
		}
		if colType, ok := col["type"].(string); ok {
			colTable.RawSetString("type", lua.LString(colType))
		}
		if nullable, ok := col["nullable"].(bool); ok {
			colTable.RawSetString("nullable", lua.LBool(nullable))
		}
		columns.RawSetInt(i+1, colTable)
	}
	meta.RawSetString("columns", columns)

	// 添加現有維度（目前為空）
	meta.RawSetString("existing_dimensions", p.luaState.NewTable())

	return meta
}

// convertLuaTableToDimensions 將 Lua 表格轉換為 Dimension 結構
func (p *Phase4Runner) convertLuaTableToDimensions(luaTable *lua.LTable) []Dimension {
	dimensions := []Dimension{}

	luaTable.ForEach(func(key lua.LValue, value lua.LValue) {
		if dimTable, ok := value.(*lua.LTable); ok {
			dimension := Dimension{}

			dimTable.ForEach(func(dimKey lua.LValue, dimValue lua.LValue) {
				keyStr := dimKey.String()
				switch keyStr {
				case "name":
					dimension.Name = dimValue.String()
				case "type":
					dimension.Type = dimValue.String()
				case "description":
					dimension.Description = dimValue.String()
				case "source_table":
					dimension.SourceTable = dimValue.String()
				case "business_use":
					dimension.BusinessUse = dimValue.String()
				case "key_fields":
					if arr, ok := dimValue.(*lua.LTable); ok {
						dimension.KeyFields = p.luaTableToStringSlice(arr)
					}
				case "attributes":
					if arr, ok := dimValue.(*lua.LTable); ok {
						dimension.Attributes = p.luaTableToStringSlice(arr)
					}
				}
			})

			dimensions = append(dimensions, dimension)
		}
	})

	return dimensions
}

// convertLuaTableToFactTables 將 Lua 表格轉換為 FactTable 結構
func (p *Phase4Runner) convertLuaTableToFactTables(luaTable *lua.LTable) []FactTable {
	factTables := []FactTable{}

	luaTable.ForEach(func(key lua.LValue, value lua.LValue) {
		if factTable, ok := value.(*lua.LTable); ok {
			fact := FactTable{}

			factTable.ForEach(func(factKey lua.LValue, factValue lua.LValue) {
				keyStr := factKey.String()
				switch keyStr {
				case "name":
					fact.Name = factValue.String()
				case "description":
					fact.Description = factValue.String()
				case "source_table":
					fact.SourceTable = factValue.String()
				case "measures":
					if arr, ok := factValue.(*lua.LTable); ok {
						fact.Measures = p.luaTableToStringSlice(arr)
					}
				case "dimensions":
					if arr, ok := factValue.(*lua.LTable); ok {
						fact.Dimensions = p.luaTableToStringSlice(arr)
					}
				}
			})

			factTables = append(factTables, fact)
		}
	})

	return factTables
}

// luaTableToStringSlice 將 Lua 表格轉換為字符串切片
func (p *Phase4Runner) luaTableToStringSlice(luaTable *lua.LTable) []string {
	result := []string{}
	luaTable.ForEach(func(key lua.LValue, value lua.LValue) {
		result = append(result, value.String())
	})
	return result
}

// generateCategorizedReport 生成按分類組織的報告
func (p *Phase4Runner) generateCategorizedReport(dimensions []Dimension, factTables []FactTable) map[string]interface{} {
	// 按照 5 個分類組織維度
	categorizedDimensions := map[string][]Dimension{
		"people":   {}, // 人 - 客戶、使用者、供應商等
		"time":     {}, // 時間 - 時間維度
		"product":  {}, // 物體 - 產品、折扣券等
		"event":    {}, // 事件 - 運輸、銷售等事件
		"location": {}, // 地點 - 地理位置
	}

	// 將維度按類型分類
	for _, dim := range dimensions {
		switch dim.Type {
		case "people":
			categorizedDimensions["people"] = append(categorizedDimensions["people"], dim)
		case "time":
			categorizedDimensions["time"] = append(categorizedDimensions["time"], dim)
		case "product":
			categorizedDimensions["product"] = append(categorizedDimensions["product"], dim)
		case "event":
			categorizedDimensions["event"] = append(categorizedDimensions["event"], dim)
		case "location":
			categorizedDimensions["location"] = append(categorizedDimensions["location"], dim)
		default:
			// 如果沒有匹配的類型，嘗試根據名稱智能分類
			if p.isPeopleDimension(dim) {
				categorizedDimensions["people"] = append(categorizedDimensions["people"], dim)
			} else if p.isTimeDimension(dim) {
				categorizedDimensions["time"] = append(categorizedDimensions["time"], dim)
			} else if p.isProductDimension(dim) {
				categorizedDimensions["product"] = append(categorizedDimensions["product"], dim)
			} else if p.isEventDimension(dim) {
				categorizedDimensions["event"] = append(categorizedDimensions["event"], dim)
			} else if p.isLocationDimension(dim) {
				categorizedDimensions["location"] = append(categorizedDimensions["location"], dim)
			} else {
				// 如果還是無法分類，放進最可能的類別
				categorizedDimensions["product"] = append(categorizedDimensions["product"], dim)
			}
		}
	}

	// 生成總結統計
	summary := p.generateCategorizedSummary(categorizedDimensions, factTables)

	return map[string]interface{}{
		"phase":         "phase4",
		"description":   "Database dimension modeling using Lua rule engine with categorized output",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"classifications": map[string]interface{}{
			"people": map[string]interface{}{
				"description": "人 - 資料模型裡面的對象 (客戶、使用者、供應商等)",
				"dimensions":  categorizedDimensions["people"],
				"count":       len(categorizedDimensions["people"]),
			},
			"time": map[string]interface{}{
				"description": "時間 - 時間維度",
				"dimensions":  categorizedDimensions["time"],
				"count":       len(categorizedDimensions["time"]),
			},
			"product": map[string]interface{}{
				"description": "物體 - 資料模型裡面的物體 (產品、折扣券等)",
				"dimensions":  categorizedDimensions["product"],
				"count":       len(categorizedDimensions["product"]),
			},
			"event": map[string]interface{}{
				"description": "事件 - 資料模型裡面的事件 (運輸、銷售等)",
				"dimensions":  categorizedDimensions["event"],
				"count":       len(categorizedDimensions["event"]),
			},
			"location": map[string]interface{}{
				"description": "地點 - 上述發生的地點",
				"dimensions":  categorizedDimensions["location"],
				"count":       len(categorizedDimensions["location"]),
			},
		},
		"fact_tables": factTables,
		"summary":     summary,
	}
}

// generateCategorizedSummary 生成分類總結
func (p *Phase4Runner) generateCategorizedSummary(categorizedDimensions map[string][]Dimension, factTables []FactTable) map[string]interface{} {
	totalDimensions := 0
	for _, dims := range categorizedDimensions {
		totalDimensions += len(dims)
	}

	return map[string]interface{}{
		"total_dimensions":  totalDimensions,
		"total_fact_tables": len(factTables),
		"classification_counts": map[string]int{
			"people":   len(categorizedDimensions["people"]),
			"time":     len(categorizedDimensions["time"]),
			"product":  len(categorizedDimensions["product"]),
			"event":    len(categorizedDimensions["event"]),
			"location": len(categorizedDimensions["location"]),
		},
		"rule_engine_info": map[string]interface{}{
			"lua_script": "knowledge/dimension_rules.lua",
			"engine":     "Gopher-Lua v1.1.1",
		},
	}
}

// 智能分類輔助方法
func (p *Phase4Runner) isPeopleDimension(dim Dimension) bool {
	name := strings.ToLower(dim.Name)
	desc := strings.ToLower(dim.Description)

	peopleKeywords := []string{"customer", "user", "person", "people", "client", "supplier", "vendor", "employee", "staff", "member"}
	for _, keyword := range peopleKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			return true
		}
	}
	return false
}

func (p *Phase4Runner) isTimeDimension(dim Dimension) bool {
	name := strings.ToLower(dim.Name)
	desc := strings.ToLower(dim.Description)

	timeKeywords := []string{"date", "time", "datetime", "timestamp", "calendar", "period", "month", "year", "day", "hour"}
	for _, keyword := range timeKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			return true
		}
	}
	return false
}

func (p *Phase4Runner) isProductDimension(dim Dimension) bool {
	name := strings.ToLower(dim.Name)
	desc := strings.ToLower(dim.Description)

	productKeywords := []string{"product", "item", "goods", "merchandise", "sku", "inventory", "catalog", "category", "coupon", "discount", "price"}
	for _, keyword := range productKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			return true
		}
	}
	return false
}

func (p *Phase4Runner) isEventDimension(dim Dimension) bool {
	name := strings.ToLower(dim.Name)
	desc := strings.ToLower(dim.Description)

	eventKeywords := []string{"order", "transaction", "sale", "purchase", "shipment", "delivery", "payment", "review", "feedback", "event", "action", "behavior"}
	for _, keyword := range eventKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			return true
		}
	}
	return false
}

func (p *Phase4Runner) isLocationDimension(dim Dimension) bool {
	name := strings.ToLower(dim.Name)
	desc := strings.ToLower(dim.Description)

	locationKeywords := []string{"location", "address", "city", "country", "region", "state", "province", "zip", "postal", "geo", "place", "area"}
	for _, keyword := range locationKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			return true
		}
	}
	return false
}

// writeOutput 寫入輸出到文件
func (p *Phase4Runner) writeOutput(data interface{}, filename string) error {
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

	log.Printf("Phase 4 dimension analysis results saved to %s", filename)
	return nil
}

// storePhase4Results 將 Phase 4 結果存儲到向量數據庫
func (p *Phase4Runner) storePhase4Results(dimensions []Dimension, factTables []FactTable) error {
	if p.knowledgeMgr == nil {
		return fmt.Errorf("knowledge manager not available")
	}

	// 創建 Phase 4 的知識數據
	phase4Knowledge := map[string]interface{}{
		"phase":                 "phase4",
		"description":           "Dimension modeling and fact table analysis using Lua rule engine",
		"database":              p.config.Database.DBName,
		"database_type":         p.config.Database.Type,
		"timestamp":             time.Now(),
		"dimensions_count":      len(dimensions),
		"fact_tables_count":     len(factTables),
		"lua_rules_file":        "knowledge/dimension_rules.lua",
		"dimensions_generated":  make([]map[string]interface{}, len(dimensions)),
		"fact_tables_generated": make([]map[string]interface{}, len(factTables)),
	}

	// 添加維度信息
	for i, dim := range dimensions {
		phase4Knowledge["dimensions_generated"].([]map[string]interface{})[i] = map[string]interface{}{
			"name":         dim.Name,
			"type":         dim.Type,
			"description":  dim.Description,
			"source_table": dim.SourceTable,
			"key_fields":   dim.KeyFields,
			"attributes":   dim.Attributes,
			"business_use": dim.BusinessUse,
		}
	}

	// 添加事實表信息
	for i, fact := range factTables {
		phase4Knowledge["fact_tables_generated"].([]map[string]interface{})[i] = map[string]interface{}{
			"name":         fact.Name,
			"description":  fact.Description,
			"source_table": fact.SourceTable,
			"measures":     fact.Measures,
			"dimensions":   fact.Dimensions,
		}
	}

	// 存儲到向量數據庫
	return p.knowledgeMgr.StorePhaseKnowledge("phase4", phase4Knowledge)
}

// Phase2ResultReader Phase 2 結果讀取器
type Phase2ResultReader struct {
	filename string
}

// NewPhase2ResultReader 創建 Phase 2 結果讀取器
func NewPhase2ResultReader(filename string) *Phase2ResultReader {
	return &Phase2ResultReader{filename: filename}
}

// GetAnalysisResults 獲取分析結果
func (r *Phase2ResultReader) GetAnalysisResults() (map[string]*LLMAnalysisResult, error) {
	data, err := os.ReadFile(r.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read Phase 2 results file: %v", err)
	}

	var result struct {
		AnalysisResults map[string]*LLMAnalysisResult `json:"analysis_results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Phase 2 results: %v", err)
	}

	return result.AnalysisResults, nil
}
