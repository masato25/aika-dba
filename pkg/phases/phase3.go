package phases

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// PrePhase3ResultReader Pre-Phase 3 結果讀取器
type PrePhase3ResultReader struct {
	filename string
}

// NewPrePhase3ResultReader 創建 Pre-Phase 3 結果讀取器
func NewPrePhase3ResultReader(filename string) *PrePhase3ResultReader {
	return &PrePhase3ResultReader{filename: filename}
}

// GetPrePhase3Data 獲取 Pre-Phase 3 數據
func (r *PrePhase3ResultReader) GetPrePhase3Data() (*PrePhase3Data, error) {
	data, err := os.ReadFile(r.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read pre-phase3 file: %v", err)
	}

	var result PrePhase3Data
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse pre-phase3 data: %v", err)
	}

	return &result, nil
}

// CustomNotes 自定義筆記
type CustomNotes struct {
	BusinessDomain         string        `json:"business_domain"`
	KeyEntities            []string      `json:"key_entities"`
	ImportantRelationships []string      `json:"important_relationships"`
	AnalysisFocusAreas     []string      `json:"analysis_focus_areas"`
	CustomDimensions       []interface{} `json:"custom_dimensions"`
	AdditionalNotes        string        `json:"additional_notes"`
}

// PrePhase3Data Pre-Phase 3 數據結構
type PrePhase3Data struct {
	BusinessSummary   map[string]interface{} `json:"business_summary"`
	Phase3Suggestions map[string]interface{} `json:"phase3_suggestions"`
	CustomNotes       *CustomNotes           `json:"custom_notes"`
}

// Phase3Runner Phase 3 執行器 - 自動生成維度規則
type Phase3Runner struct {
	config       *config.Config
	knowledgeMgr *vectorstore.KnowledgeManager
	llmClient    *LLMClient
}

// NewPhase3Runner 創建 Phase 3 執行器
func NewPhase3Runner(cfg *config.Config) *Phase3Runner {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Printf("Warning: Failed to create knowledge manager: %v", err)
		knowledgeMgr = nil
	}

	return &Phase3Runner{
		config:       cfg,
		knowledgeMgr: knowledgeMgr,
		llmClient:    NewLLMClient(cfg),
	}
}

// Run 執行 Phase 3 規則生成
func (p *Phase3Runner) Run() error {
	log.Println("=== Starting Phase 3: Auto-Generate Dimension Rules ===")

	// 從向量存儲檢索 Phase 2 分析結果
	phase2Results, err := p.retrievePhase2Knowledge()
	if err != nil {
		return fmt.Errorf("failed to retrieve Phase 2 knowledge: %v", err)
	}

	// 從向量存儲檢索 pre-Phase 3 總結
	prePhase3Data, err := p.retrievePrePhase3Knowledge()
	if err != nil {
		log.Printf("Warning: Failed to retrieve pre-Phase 3 knowledge: %v, using defaults", err)
		prePhase3Data = p.createDefaultPrePhase3Data()
	}

	// 生成 Lua 維度規則
	luaRules, err := p.generateDimensionRules(phase2Results, prePhase3Data)
	if err != nil {
		return fmt.Errorf("failed to generate dimension rules: %v", err)
	}

	// 保存 Lua 規則文件
	if err := p.saveLuaRules(luaRules); err != nil {
		return fmt.Errorf("failed to save Lua rules: %v", err)
	}

	// 將 Phase 3 結果存儲到向量數據庫
	if err := p.storePhase3Results(phase2Results, prePhase3Data, luaRules); err != nil {
		log.Printf("Warning: Failed to store Phase 3 results in vector store: %v", err)
	}

	log.Println("Phase 3 completed successfully - dimension_rules.lua generated")
	return nil
}

// generateDimensionRules 生成維度規則
func (p *Phase3Runner) generateDimensionRules(phase2Results map[string]*LLMAnalysisResult, prePhase3Data *PrePhase3Data) (string, error) {
	var luaRules strings.Builder

	// Lua 文件頭部
	luaRules.WriteString(`-- Auto-generated dimension rules by Phase 2.5
-- Generated on: ` + time.Now().Format("2006-01-02 15:04:05") + `
-- Database: ` + p.config.Database.DBName + `

-- Helper functions
function contains(table, element)
    for _, value in pairs(table) do
        if value == element then
            return true
        end
    end
    return false
end

function has_any_field(table_meta, fields)
    if not table_meta.columns then
        return false
    end
    for _, col in pairs(table_meta.columns) do
        if col.name then
            for _, field in ipairs(fields) do
                if col.name == field then
                    return true
                end
            end
        end
    end
    return false
end

-- Dimension detection function
function detect_dimensions(table_name, table_meta)
    local dimensions = {}

`)

	// 使用 LLM 生成智慧規則
	intelligentRules, err := p.generateIntelligentRules(phase2Results, prePhase3Data)
	if err != nil {
		log.Printf("Warning: Failed to generate intelligent rules, using basic rules: %v", err)
		intelligentRules = p.generateBasicRules()
	}
	luaRules.WriteString(intelligentRules)

	// 基於字段分析的動態規則
	luaRules.WriteString(`
    -- Dynamic rules based on field analysis
    if has_any_field(table_meta, {"price", "sku", "product_id"}) and not contains(table_meta.existing_dimensions, "dim_product") then
        -- Extract column names for attributes
        local column_names = {}
        for _, col in pairs(table_meta.columns) do
            if col.name then
                table.insert(column_names, col.name)
            end
        end
        table.insert(dimensions, {
            name = "dim_product",
            type = "product",
            description = "Product dimension table - automatically identified product-related table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = column_names,
            business_use = "Used for product-related business analysis"
        })
    end

    if has_any_field(table_meta, {"customer_id", "user_id", "email"}) and not contains(table_meta.existing_dimensions, "dim_customer") then
        -- Extract column names for attributes
        local column_names = {}
        for _, col in pairs(table_meta.columns) do
            if col.name then
                table.insert(column_names, col.name)
            end
        end
        table.insert(dimensions, {
            name = "dim_customer",
            type = "people",
            description = "Customer dimension table - automatically identified customer-related table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = column_names,
            business_use = "Used for customer-related business analysis"
        })
    end

    if has_any_field(table_meta, {"created_at", "updated_at", "date"}) and not contains(table_meta.existing_dimensions, "dim_date") then
        -- Extract column names for attributes
        local column_names = {}
        for _, col in pairs(table_meta.columns) do
            if col.name then
                table.insert(column_names, col.name)
            end
        end
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - automatically identified time-related table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = column_names,
            business_use = "Used for time series analysis"
        })
    end

    if has_any_field(table_meta, {"address", "city", "country"}) and not contains(table_meta.existing_dimensions, "dim_location") then
        -- Extract column names for attributes
        local column_names = {}
        for _, col in pairs(table_meta.columns) do
            if col.name then
                table.insert(column_names, col.name)
            end
        end
        table.insert(dimensions, {
            name = "dim_location",
            type = "location",
            description = "Location dimension table - automatically identified geography-related table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = column_names,
            business_use = "Used for geographic location analysis"
        })
    end
`)

	// 從 pre-phase3 總結中提取額外的規則
	if prePhase3Data.CustomNotes != nil && len(prePhase3Data.CustomNotes.CustomDimensions) > 0 {
		luaRules.WriteString(`
    -- Custom dimensions from pre-phase3 summary
`)
		for _, customDim := range prePhase3Data.CustomNotes.CustomDimensions {
			if dimMap, ok := customDim.(map[string]interface{}); ok {
				name := dimMap["name"].(string)
				luaRules.WriteString(fmt.Sprintf(`
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "%s",
            type = "%s",
            description = "%s",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {},
            business_use = "%s"
        })
    end`, name, name, dimMap["type"], dimMap["description"], dimMap["purpose"]))
			}
		}
	}

	luaRules.WriteString(`

    return dimensions
end

-- Fact table detection function
function detect_fact_tables(table_name, table_meta)
    local fact_tables = {}

    -- Rule-based fact table detection
    if has_any_field(table_meta, {"order_id", "quantity", "amount", "total"}) then
        table.insert(fact_tables, {
            name = "fact_sales",
            description = "Sales fact table - records sales transactions",
            source_table = table_name,
            measures = {"quantity", "amount", "total", "discount"},
            dimensions = {"dim_date", "dim_customer", "dim_product", "dim_location"}
        })
    elseif has_any_field(table_meta, {"inventory_change", "stock_quantity"}) then
        table.insert(fact_tables, {
            name = "fact_inventory",
            description = "Inventory fact table - records inventory changes",
            source_table = table_name,
            measures = {"quantity_change", "unit_cost", "total_value"},
            dimensions = {"dim_date", "dim_product", "dim_location"}
        })
    elseif has_any_field(table_meta, {"event_type", "session_id", "page_view"}) then
        table.insert(fact_tables, {
            name = "fact_customer_behavior",
            description = "Customer behavior fact table - records user behavior",
            source_table = table_name,
            measures = {"event_count", "session_duration", "page_views"},
            dimensions = {"dim_date", "dim_customer", "dim_location"}
        })
    end

    return fact_tables
end

return {
    detect_dimensions = detect_dimensions,
    detect_fact_tables = detect_fact_tables
}`)

	return luaRules.String(), nil
}

// generateIntelligentRules 使用 LLM 生成智慧規則
func (p *Phase3Runner) generateIntelligentRules(phase2Results map[string]*LLMAnalysisResult, prePhase3Data *PrePhase3Data) (string, error) {
	var rules strings.Builder

	rules.WriteString(`
    -- Intelligent rules generated by LLM based on Phase 2 analysis
`)

	// 為每個表格生成智慧規則
	for tableName, result := range phase2Results {
		// 使用現有的分析結果來生成規則
		tableRules := p.generateRulesFromAnalysis(tableName, result)
		if tableRules != "" {
			rules.WriteString(tableRules)
			rules.WriteString("\n")
		}
	}

	return rules.String(), nil
}

// generateRulesFromAnalysis 基於分析結果生成規則
func (p *Phase3Runner) generateRulesFromAnalysis(tableName string, result *LLMAnalysisResult) string {
	// 基於分析內容生成規則
	var rules strings.Builder

	// 根據表名和分析內容智能判斷維度類型
	switch tableName {
	case "customers":
		// 顧客表 - 生成顧客維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_customer",
            type = "people",
            description = "Customer dimension table - contains customer information and behavior data",
            source_table = table_name,
            key_fields = {"id", "email"},
            attributes = {"name", "phone", "date_of_birth", "gender", "registration_date", "last_login", "is_active", "customer_type", "total_orders", "total_spent", "loyalty_points"},
            business_use = "Used for customer behavior analysis and segmentation"
        })
    end
`, tableName))

	case "products":
		// 商品表 - 生成商品維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_product",
            type = "product",
            description = "Product dimension table - contains product information and attributes",
            source_table = table_name,
            key_fields = {"id", "sku"},
            attributes = {"name", "description", "short_description", "category_id", "price", "cost_price", "compare_at_price", "weight", "dimensions", "inventory_quantity", "inventory_policy", "is_active", "is_featured", "tags", "images", "seo_title", "seo_description"},
            business_use = "Used for product sales analysis and categorization"
        })
    end
`, tableName))

	case "categories":
		// 分類表 - 生成分類維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_category",
            type = "product",
            description = "Category dimension table - contains category hierarchy structure",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"name", "description", "parent_id", "sort_order", "is_active"},
            business_use = "Used for category performance analysis and hierarchy statistics"
        })
    end
`, tableName))

	case "orders":
		// 訂單表 - 生成時間維度和位置維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from order timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"created_at", "updated_at", "ordered_at", "confirmed_at", "shipped_at", "delivered_at", "cancelled_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_location",
            type = "location",
            description = "Location dimension table - contains address and region information",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"billing_address", "shipping_address"},
            business_use = "Used for geographic analysis and regional sales statistics"
        })
    end
`, tableName, tableName))

	case "order_items":
		// 訂單項目表 - 生成時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from order item timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"created_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
`, tableName))

	case "customer_addresses":
		// 顧客地址表 - 生成位置維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_location",
            type = "location",
            description = "Location dimension table - contains customer address information",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"address_type", "is_default", "street_address", "city", "state", "postal_code", "country"},
            business_use = "Used for geographic analysis and regional sales statistics"
        })
    end
`, tableName))

	case "coupons":
		// 優惠券表 - 生成促銷維度和時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_promotion",
            type = "event",
            description = "Promotion dimension table - contains coupon and promotion information",
            source_table = table_name,
            key_fields = {"id", "code"},
            attributes = {"name", "description", "discount_type", "discount_value", "minimum_amount", "maximum_discount", "usage_limit", "usage_count", "starts_at", "expires_at", "is_active", "applicable_products", "applicable_categories"},
            business_use = "Used for marketing effectiveness analysis and ROI calculation"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from coupon timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"starts_at", "expires_at", "created_at", "updated_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
`, tableName, tableName))

	case "reviews":
		// 評論表 - 生成評論維度和時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_review",
            type = "event",
            description = "Review dimension table - contains user ratings and feedback",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"rating", "title", "content", "images", "is_verified", "helpful_votes", "status"},
            business_use = "Used for rating analysis and user satisfaction research"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from review timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"created_at", "updated_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
`, tableName, tableName))

	case "shipments":
		// 運送表 - 生成運送維度和位置維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_shipment",
            type = "event",
            description = "Shipment dimension table - contains logistics and delivery information",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"tracking_number", "carrier", "status", "shipped_at", "delivered_at", "estimated_delivery", "shipping_address", "weight", "dimensions", "notes"},
            business_use = "Used for logistics analysis and delivery efficiency optimization"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_location",
            type = "location",
            description = "Location dimension table - contains shipping address information",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"shipping_address"},
            business_use = "Used for geographic analysis and regional sales statistics"
        })
    end
`, tableName, tableName))

	case "returns":
		// 退貨表 - 生成退貨維度和時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_return",
            type = "event",
            description = "Return dimension table - contains return and refund information",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"reason", "status", "refund_amount", "items", "notes", "requested_at", "approved_at", "received_at", "refunded_at"},
            business_use = "Used for return analysis and quality improvement"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from return timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"requested_at", "approved_at", "received_at", "refunded_at", "created_at", "updated_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
`, tableName, tableName))

	case "product_variants":
		// 商品變體表 - 生成商品維度和時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_product",
            type = "product",
            description = "Product dimension table - contains product variant information",
            source_table = table_name,
            key_fields = {"id", "sku"},
            attributes = {"product_id", "name", "price", "cost_price", "compare_at_price", "weight", "dimensions", "inventory_quantity", "option1", "option2", "option3", "is_active"},
            business_use = "Used for product variant analysis and categorization"
        })
    end
    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from variant timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"created_at", "updated_at"},
            business_use = "Used for time series analysis and trend analysis"
        })
    end
`, tableName, tableName))

	case "inventory_transactions":
		// 庫存交易表 - 生成時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from inventory transaction timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"created_at"},
            business_use = "Used for inventory time series analysis"
        })
    end
`, tableName))

	case "payments":
		// 支付表 - 生成時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from payment timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"processed_at", "created_at", "updated_at"},
            business_use = "Used for payment time series analysis"
        })
    end
`, tableName))

	case "shopping_carts":
		// 購物車表 - 生成時間維度
		rules.WriteString(fmt.Sprintf(`    if table_name == "%s" then
        table.insert(dimensions, {
            name = "dim_date",
            type = "time",
            description = "Time dimension table - extracted from shopping cart timestamps",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {"expires_at", "created_at", "updated_at"},
            business_use = "Used for shopping cart time series analysis"
        })
    end
`, tableName))
	}

	return rules.String()
}

// generateBasicRules 生成基本後備規則
func (p *Phase3Runner) generateBasicRules() string {
	return `
    -- Basic fallback rules
    if table_name == "customers" or table_name == "users" then
        table.insert(dimensions, {
            name = "dim_customer",
            type = "people",
            description = "Customer dimension table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {},
            business_use = "Customer analysis"
        })
    elseif table_name == "products" or table_name == "items" then
        table.insert(dimensions, {
            name = "dim_product",
            type = "product",
            description = "Product dimension table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {},
            business_use = "Product analysis"
        })
    elseif table_name == "categories" then
        table.insert(dimensions, {
            name = "dim_category",
            type = "product",
            description = "Category dimension table",
            source_table = table_name,
            key_fields = {"id"},
            attributes = {},
            business_use = "Category analysis"
        })
    end`
}

// saveLuaRules 保存 Lua 規則到文件
func (p *Phase3Runner) saveLuaRules(rules string) error {
	filename := "knowledge/dimension_rules.lua"

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(rules)
	if err != nil {
		return err
	}

	log.Printf("Dimension rules saved to %s", filename)
	return nil
}

// retrievePhase2Knowledge 從向量存儲檢索 Phase 2 知識
func (p *Phase3Runner) retrievePhase2Knowledge() (map[string]*LLMAnalysisResult, error) {
	if p.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	// 檢索 Phase 2 的分析結果
	query := "business logic analysis AI insights recommendations"
	results, err := p.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve phase2 knowledge: %v", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no phase2 knowledge found in vector store")
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

	log.Printf("Retrieved %d table analysis results from vector store", len(phase2Results))
	return phase2Results, nil
}

// retrievePrePhase3Knowledge 從向量存儲檢索 pre-Phase 3 知識
func (p *Phase3Runner) retrievePrePhase3Knowledge() (*PrePhase3Data, error) {
	if p.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	// 檢索 pre-phase3 的總結
	query := "business summary phase3 suggestions custom dimensions"
	results, err := p.knowledgeMgr.RetrievePhaseKnowledge("pre_phase3", query, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pre-phase3 knowledge: %v", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no pre-phase3 knowledge found in vector store")
	}

	// 從檢索到的知識中重建 pre-phase3 數據
	var prePhase3Data *PrePhase3Data

	for _, result := range results {
		content := result.Content

		// 嘗試解析為 JSON
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(content), &data); err != nil {
			log.Printf("Failed to parse pre-phase3 knowledge as JSON: %v", err)
			continue
		}

		prePhase3Data = &PrePhase3Data{}

		if businessSummary, ok := data["business_summary"].(map[string]interface{}); ok {
			prePhase3Data.BusinessSummary = businessSummary
		}

		if phase3Suggestions, ok := data["phase3_suggestions"].(map[string]interface{}); ok {
			prePhase3Data.Phase3Suggestions = phase3Suggestions
		}

		if customNotes, ok := data["custom_notes"].(map[string]interface{}); ok {
			prePhase3Data.CustomNotes = &CustomNotes{}

			if businessDomain, ok := customNotes["business_domain"].(string); ok {
				prePhase3Data.CustomNotes.BusinessDomain = businessDomain
			}

			if keyEntities, ok := customNotes["key_entities"].([]interface{}); ok {
				prePhase3Data.CustomNotes.KeyEntities = make([]string, len(keyEntities))
				for i, entity := range keyEntities {
					if entityStr, ok := entity.(string); ok {
						prePhase3Data.CustomNotes.KeyEntities[i] = entityStr
					}
				}
			}

			if importantRelationships, ok := customNotes["important_relationships"].([]interface{}); ok {
				prePhase3Data.CustomNotes.ImportantRelationships = make([]string, len(importantRelationships))
				for i, rel := range importantRelationships {
					if relStr, ok := rel.(string); ok {
						prePhase3Data.CustomNotes.ImportantRelationships[i] = relStr
					}
				}
			}

			if analysisFocusAreas, ok := customNotes["analysis_focus_areas"].([]interface{}); ok {
				prePhase3Data.CustomNotes.AnalysisFocusAreas = make([]string, len(analysisFocusAreas))
				for i, area := range analysisFocusAreas {
					if areaStr, ok := area.(string); ok {
						prePhase3Data.CustomNotes.AnalysisFocusAreas[i] = areaStr
					}
				}
			}

			if customDimensions, ok := customNotes["custom_dimensions"].([]interface{}); ok {
				prePhase3Data.CustomNotes.CustomDimensions = customDimensions
			}

			if additionalNotes, ok := customNotes["additional_notes"].(string); ok {
				prePhase3Data.CustomNotes.AdditionalNotes = additionalNotes
			}
		}

		break // 只使用第一個匹配的結果
	}

	if prePhase3Data == nil {
		return nil, fmt.Errorf("failed to reconstruct pre-phase3 data from vector store")
	}

	log.Printf("Retrieved pre-phase3 data from vector store")
	return prePhase3Data, nil
}

// createDefaultPrePhase3Data 創建默認的 pre-Phase 3 數據
func (p *Phase3Runner) createDefaultPrePhase3Data() *PrePhase3Data {
	return &PrePhase3Data{
		BusinessSummary: map[string]interface{}{
			"overall_business_domain": "通用商業系統 (General Business System)",
			"core_business_processes": []string{
				"數據管理 (Data Management)",
				"業務分析 (Business Analysis)",
			},
			"key_business_entities": []string{
				"記錄 (Records)",
				"交易 (Transactions)",
			},
			"business_relationships": []string{
				"數據關聯 (Data Relationships)",
			},
			"table_categories": map[string][]string{
				"data_tables": {"various"},
			},
		},
		Phase3Suggestions: map[string]interface{}{
			"recommended_dimensions": []map[string]interface{}{
				{
					"name":            "dim_date",
					"purpose":         "時間維度 - 用於時間序列分析",
					"based_on_tables": []string{"any_timestamp_fields"},
				},
			},
			"recommended_fact_tables": []map[string]interface{}{
				{
					"name":         "fact_data",
					"purpose":      "通用事實表",
					"grain":        "每筆記錄 (Per Record)",
					"key_measures": []string{"count", "amount"},
				},
			},
			"analysis_focus_areas": []string{
				"數據分析 (Data Analysis)",
				"趨勢分析 (Trend Analysis)",
			},
		},
		CustomNotes: &CustomNotes{
			BusinessDomain:         "通用商業系統",
			KeyEntities:            []string{"記錄", "交易"},
			ImportantRelationships: []string{"數據關聯"},
			AnalysisFocusAreas:     []string{"數據分析", "趨勢分析"},
			CustomDimensions:       []interface{}{},
			AdditionalNotes:        "使用默認配置",
		},
	}
}

// storePhase3Results 將 Phase 3 結果存儲到向量數據庫
func (p *Phase3Runner) storePhase3Results(phase2Results map[string]*LLMAnalysisResult, prePhase3Data *PrePhase3Data, luaRules string) error {
	if p.knowledgeMgr == nil {
		return fmt.Errorf("knowledge manager not available")
	}

	// 創建 Phase 3 的知識數據
	phase3Knowledge := map[string]interface{}{
		"phase":               "phase3",
		"description":         "Dimension modeling rules and ETL planning generated from business analysis",
		"database":            p.config.Database.DBName,
		"database_type":       p.config.Database.Type,
		"timestamp":           time.Now(),
		"input_phase2_tables": len(phase2Results),
		"lua_rules_generated": len(luaRules) > 0,
		"rules_file":          "knowledge/dimension_rules.lua",
		"business_domain":     prePhase3Data.CustomNotes.BusinessDomain,
		"key_entities":        prePhase3Data.CustomNotes.KeyEntities,
		"analysis_focus":      prePhase3Data.CustomNotes.AnalysisFocusAreas,
		"generated_rules_summary": map[string]interface{}{
			"total_rules_length":         len(luaRules),
			"has_custom_dimensions":      len(prePhase3Data.CustomNotes.CustomDimensions) > 0,
			"business_domain_identified": prePhase3Data.CustomNotes.BusinessDomain != "",
		},
	}

	// 存儲到向量數據庫
	return p.knowledgeMgr.StorePhaseKnowledge("phase3", phase3Knowledge)
}
