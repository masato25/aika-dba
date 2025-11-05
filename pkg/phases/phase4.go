package phases

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/masato25/aika-dba/config"
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
	reader       *Phase1ResultReader
	phase2Reader *Phase2ResultReader
	luaState     *lua.LState
}

// NewPhase4Runner 創建 Phase 4 執行器
func NewPhase4Runner(cfg *config.Config, db *sql.DB) *Phase4Runner {
	return &Phase4Runner{
		config:       cfg,
		db:           db,
		reader:       NewPhase1ResultReader("knowledge/phase1_analysis.json"),
		phase2Reader: NewPhase2ResultReader("knowledge/phase2_analysis.json"),
	}
}

// Run 執行 Phase 4 維度建模分析（使用 Lua 規則引擎）
func (p *Phase4Runner) Run() error {
	log.Println("=== Starting Phase 4: Lua Rule Engine Dimension Modeling ===")

	// 初始化 Lua VM
	if err := p.initLuaVM(); err != nil {
		return fmt.Errorf("failed to initialize Lua VM: %v", err)
	}
	defer p.luaState.Close()

	// 載入 Phase 2 分析結果
	phase2Results, err := p.phase2Reader.GetAnalysisResults()
	if err != nil {
		return fmt.Errorf("failed to load Phase 2 results: %v", err)
	}

	// 使用 Lua 規則引擎生成維度
	dimensions, factTables, err := p.executeLuaRules(phase2Results)
	if err != nil {
		return fmt.Errorf("failed to execute Lua rules: %v", err)
	}

	// 生成維度建模報告
	report := map[string]interface{}{
		"phase":         "phase4",
		"description":   "Database dimension modeling using Lua rule engine",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"dimensions":    dimensions,
		"fact_tables":   factTables,
		"summary":       p.generateSummary(dimensions, factTables),
	}

	// 保存報告
	return p.writeOutput(report, "knowledge/phase4_dimensions.json")
}

// initLuaVM 初始化 Lua 虛擬機
func (p *Phase4Runner) initLuaVM() error {
	p.luaState = lua.NewState()
	return p.luaState.DoFile("knowledge/dimension_rules.lua")
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

	// 從 Phase 1 結果獲取欄位信息
	tableAnalysis, err := p.reader.GetTableAnalysis(tableName)
	if err != nil {
		log.Printf("Warning: Failed to get table analysis for %s: %v", tableName, err)
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

// generateSummary 生成總結
func (p *Phase4Runner) generateSummary(dimensions []Dimension, factTables []FactTable) map[string]interface{} {
	typeCount := make(map[string]int)
	totalDimensions := len(dimensions)
	totalFactTables := len(factTables)

	for _, dim := range dimensions {
		typeCount[dim.Type]++
	}

	return map[string]interface{}{
		"total_dimensions":   totalDimensions,
		"total_fact_tables":  totalFactTables,
		"dimensions_by_type": typeCount,
		"rule_engine_info": map[string]interface{}{
			"lua_script": "knowledge/dimension_rules.lua",
			"engine":     "Gopher-Lua v1.1.1",
		},
	}
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
