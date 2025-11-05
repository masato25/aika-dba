package phases

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
)

// Phase3Runner Phase 3 執行器 - 自動生成維度規則
type Phase3Runner struct {
	config          *config.Config
	phase2Reader    *Phase2ResultReader
	prePhase3Reader *PrePhase3ResultReader
	llmClient       *LLMClient
}

// NewPhase3Runner 創建 Phase 3 執行器
func NewPhase3Runner(cfg *config.Config) *Phase3Runner {
	return &Phase3Runner{
		config:          cfg,
		phase2Reader:    NewPhase2ResultReader("knowledge/phase2_analysis.json"),
		prePhase3Reader: NewPrePhase3ResultReader("knowledge/pre_phase3_summary.json"),
		llmClient:       NewLLMClient(cfg),
	}
}

// Run 執行 Phase 3 規則生成
func (p *Phase3Runner) Run() error {
	log.Println("=== Starting Phase 3: Auto-Generate Dimension Rules ===")

	// 載入 Phase 2 分析結果
	phase2Results, err := p.phase2Reader.GetAnalysisResults()
	if err != nil {
		return fmt.Errorf("failed to load Phase 2 results: %v", err)
	}

	// 載入 pre-Phase 3 總結
	prePhase3Data, err := p.prePhase3Reader.GetPrePhase3Data()
	if err != nil {
		return fmt.Errorf("failed to load pre-Phase 3 summary: %v", err)
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

// PrePhase3ResultReader Pre-Phase 3 結果讀取器
type PrePhase3ResultReader struct {
	filename string
}

// PrePhase3Data Pre-Phase 3 數據結構
type PrePhase3Data struct {
	BusinessSummary   map[string]interface{} `json:"business_summary"`
	Phase3Suggestions map[string]interface{} `json:"phase3_suggestions"`
	CustomNotes       *CustomNotes           `json:"custom_notes"`
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
