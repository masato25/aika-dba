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

// Phase3Runner Phase 3 執行器 - 維度建模分析
type Phase3Runner struct {
	config       *config.Config
	db           *sql.DB
	reader       *Phase1ResultReader
	phase2Reader *Phase2ResultReader
}

// NewPhase3Runner 創建 Phase 3 執行器
func NewPhase3Runner(cfg *config.Config, db *sql.DB) *Phase3Runner {
	return &Phase3Runner{
		config:       cfg,
		db:           db,
		reader:       NewPhase1ResultReader("knowledge/phase1_analysis.json"),
		phase2Reader: NewPhase2ResultReader("knowledge/phase2_analysis.json"),
	}
}

// Run 執行 Phase 3 維度建模分析
func (p *Phase3Runner) Run() error {
	log.Println("=== Starting Phase 3: Dimension Modeling Analysis ===")

	// 載入 Phase 2 分析結果
	phase2Results, err := p.phase2Reader.GetAnalysisResults()
	if err != nil {
		return fmt.Errorf("failed to load Phase 2 results: %v", err)
	}

	// 生成維度分析
	dimensions, err := p.analyzeDimensions(phase2Results)
	if err != nil {
		return fmt.Errorf("failed to analyze dimensions: %v", err)
	}

	// 生成維度建模報告
	report := map[string]interface{}{
		"phase":         "phase3",
		"description":   "Database dimension modeling for BI and data warehousing",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"dimensions":    dimensions,
		"summary":       p.generateSummary(dimensions),
	}

	// 保存報告
	return p.writeOutput(report, "knowledge/phase3_dimensions.json")
}

// analyzeDimensions 分析維度
func (p *Phase3Runner) analyzeDimensions(phase2Results map[string]*LLMAnalysisResult) ([]Dimension, error) {
	dimensions := []Dimension{}

	// 分析每個表格並識別維度
	for tableName, result := range phase2Results {
		tableDimensions := p.analyzeTableDimensions(tableName, result)
		dimensions = append(dimensions, tableDimensions...)
	}

	return dimensions, nil
}

// analyzeTableDimensions 分析單個表格的維度
func (p *Phase3Runner) analyzeTableDimensions(tableName string, result *LLMAnalysisResult) []Dimension {
	dimensions := []Dimension{}

	switch tableName {
	case "customers":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_customer",
			Type:        "people",
			Description: "顧客維度表 - 包含顧客的基本信息、會員等級、消費行為等",
			SourceTable: "customers",
			KeyFields:   []string{"id", "uuid", "email"},
			Attributes:  []string{"name", "phone", "date_of_birth", "gender", "registration_date", "last_login", "is_active", "customer_type", "total_orders", "total_spent", "loyalty_points"},
			BusinessUse: "用於分析顧客行為、會員分群、個性化推薦、顧客價值分析",
		})

	case "products":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_product",
			Type:        "product",
			Description: "商品維度表 - 包含商品的基本信息、價格、庫存、分類等",
			SourceTable: "products",
			KeyFields:   []string{"id", "sku"},
			Attributes:  []string{"name", "description", "price", "cost_price", "compare_at_price", "weight", "dimensions", "inventory_quantity", "inventory_policy", "is_active", "is_featured", "tags", "seo_title", "seo_description"},
			BusinessUse: "用於分析商品銷售表現、庫存管理、價格策略、商品分類統計",
		})

	case "categories":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_category",
			Type:        "product",
			Description: "商品分類維度表 - 包含商品分類的層次結構和屬性",
			SourceTable: "categories",
			KeyFields:   []string{"id"},
			Attributes:  []string{"name", "description", "parent_id", "sort_order", "is_active"},
			BusinessUse: "用於分析商品分類表現、分類層次統計、分類銷售趨勢",
		})

	case "product_variants":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_product_variant",
			Type:        "product",
			Description: "商品變體維度表 - 包含商品的具體規格變體信息",
			SourceTable: "product_variants",
			KeyFields:   []string{"id", "sku"},
			Attributes:  []string{"product_id", "name", "price", "cost_price", "compare_at_price", "weight", "dimensions", "inventory_quantity", "option1", "option2", "option3", "is_active"},
			BusinessUse: "用於分析商品變體銷售、庫存管理、規格偏好分析",
		})

	case "customer_addresses":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_location",
			Type:        "location",
			Description: "地理位置維度表 - 包含顧客地址、配送地區等地理信息",
			SourceTable: "customer_addresses",
			KeyFields:   []string{"id", "customer_id"},
			Attributes:  []string{"address_type", "is_default", "street_address", "city", "state", "postal_code", "country"},
			BusinessUse: "用於分析地理銷售分佈、配送區域優化、地區顧客行為",
		})

	case "orders":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_date",
			Type:        "time",
			Description: "時間維度表 - 從訂單中提取的時間相關信息",
			SourceTable: "orders",
			KeyFields:   []string{"id"},
			Attributes:  []string{"created_at", "updated_at", "order_date", "confirmed_at", "shipped_at", "delivered_at", "cancelled_at"},
			BusinessUse: "用於時間序列分析、銷售趨勢、季節性分析、訂單處理時間統計",
		})

	case "coupons":
		dimensions = append(dimensions, Dimension{
			Name:        "dim_promotion",
			Type:        "event",
			Description: "行銷活動維度表 - 包含優惠券、促銷活動等行銷信息",
			SourceTable: "coupons",
			KeyFields:   []string{"id", "code"},
			Attributes:  []string{"name", "description", "discount_type", "discount_value", "minimum_amount", "maximum_discount", "usage_limit", "usage_count", "starts_at", "expires_at", "is_active", "applicable_products", "applicable_categories"},
			BusinessUse: "用於分析促銷效果、優惠券使用率、行銷ROI、顧客響應度",
		})
	}

	// 從分析結果中動態識別其他維度
	if strings.Contains(result.Analysis, "評論") || strings.Contains(result.Analysis, "review") {
		dimensions = append(dimensions, Dimension{
			Name:        "dim_review",
			Type:        "event",
			Description: "評論事件維度表 - 包含產品評論、評價等用戶反饋信息",
			SourceTable: "reviews",
			KeyFields:   []string{"id"},
			Attributes:  []string{"product_id", "customer_id", "order_id", "rating", "title", "content", "images", "helpful_votes", "status", "created_at", "updated_at"},
			BusinessUse: "用於分析產品評價趨勢、顧客滿意度、評論情感分析",
		})
	}

	if strings.Contains(result.Analysis, "運送") || strings.Contains(result.Analysis, "shipment") {
		dimensions = append(dimensions, Dimension{
			Name:        "dim_shipment",
			Type:        "event",
			Description: "運送事件維度表 - 包含訂單運送、物流信息",
			SourceTable: "shipments",
			KeyFields:   []string{"id", "order_id"},
			Attributes:  []string{"tracking_number", "carrier", "status", "shipping_address", "weight", "dimensions", "estimated_delivery", "delivered_at", "created_at", "updated_at"},
			BusinessUse: "用於分析物流效率、運送時間、配送成本、地區配送統計",
		})
	}

	if strings.Contains(result.Analysis, "退貨") || strings.Contains(result.Analysis, "return") {
		dimensions = append(dimensions, Dimension{
			Name:        "dim_return",
			Type:        "event",
			Description: "退貨事件維度表 - 包含退貨請求、退款信息",
			SourceTable: "returns",
			KeyFields:   []string{"id", "order_id"},
			Attributes:  []string{"customer_id", "reason", "status", "refund_amount", "refunded_items", "requested_at", "approved_at", "received_at", "refunded_at"},
			BusinessUse: "用於分析退貨率、退貨原因、退款處理效率、產品質量問題",
		})
	}

	return dimensions
}

// generateSummary 生成維度分析總結
func (p *Phase3Runner) generateSummary(dimensions []Dimension) map[string]interface{} {
	typeCount := make(map[string]int)
	totalDimensions := len(dimensions)

	for _, dim := range dimensions {
		typeCount[dim.Type]++
	}

	return map[string]interface{}{
		"total_dimensions":   totalDimensions,
		"dimensions_by_type": typeCount,
		"recommended_fact_tables": []map[string]interface{}{
			{
				"name":        "fact_sales",
				"description": "銷售事實表 - 記錄每筆銷售交易的詳細信息",
				"measures":    []string{"order_amount", "quantity", "discount_amount", "tax_amount", "shipping_cost"},
				"dimensions":  []string{"dim_date", "dim_customer", "dim_product", "dim_location", "dim_promotion"},
			},
			{
				"name":        "fact_inventory",
				"description": "庫存事實表 - 記錄庫存變動和商品流轉信息",
				"measures":    []string{"quantity_change", "unit_cost", "total_value"},
				"dimensions":  []string{"dim_date", "dim_product", "dim_location"},
			},
			{
				"name":        "fact_customer_behavior",
				"description": "顧客行為事實表 - 記錄顧客的各種行為事件",
				"measures":    []string{"event_count", "session_duration", "page_views"},
				"dimensions":  []string{"dim_date", "dim_customer", "dim_product", "dim_location"},
			},
		},
	}
}

// writeOutput 寫入輸出到文件
func (p *Phase3Runner) writeOutput(data interface{}, filename string) error {
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

	log.Printf("Phase 3 dimension analysis results saved to %s", filename)
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
