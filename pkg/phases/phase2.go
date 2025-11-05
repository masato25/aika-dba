package phases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/mcp"
)

// Dimension 維度定義
type Dimension struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`        // people, time, product, event, location
	Description string   `json:"description"`
	SourceTable string   `json:"source_table"`
	KeyFields   []string `json:"key_fields"`
	Attributes  []string `json:"attributes"`
	BusinessUse string   `json:"business_use"`
}

// DimensionAnalyzer 維度分析器
type DimensionAnalyzer struct{}

// NewDimensionAnalyzer 創建維度分析器
func NewDimensionAnalyzer() *DimensionAnalyzer {
	return &DimensionAnalyzer{}
}

// AnalyzeDimensions 分析維度
func (da *DimensionAnalyzer) AnalyzeDimensions(tableResults map[string]*LLMAnalysisResult) []Dimension {
	dimensions := []Dimension{}

	// 分析每個表格並識別維度
	for tableName, result := range tableResults {
		tableDimensions := da.analyzeTableDimensions(tableName, result)
		dimensions = append(dimensions, tableDimensions...)
	}

	return dimensions
}

// analyzeTableDimensions 分析單個表格的維度
func (da *DimensionAnalyzer) analyzeTableDimensions(tableName string, result *LLMAnalysisResult) []Dimension {
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

// GenerateSummary 生成維度分析總結
func (da *DimensionAnalyzer) GenerateSummary(dimensions []Dimension) map[string]interface{} {
	typeCount := make(map[string]int)
	totalDimensions := len(dimensions)

	for _, dim := range dimensions {
		typeCount[dim.Type]++
	}

	return map[string]interface{}{
		"total_dimensions": totalDimensions,
		"dimensions_by_type": typeCount,
		"recommended_fact_tables": []map[string]interface{}{
			{
				"name": "fact_sales",
				"description": "銷售事實表 - 記錄每筆銷售交易的詳細信息",
				"measures": []string{"order_amount", "quantity", "discount_amount", "tax_amount", "shipping_cost"},
				"dimensions": []string{"dim_date", "dim_customer", "dim_product", "dim_location", "dim_promotion"},
			},
			{
				"name": "fact_inventory",
				"description": "庫存事實表 - 記錄庫存變動和商品流轉信息",
				"measures": []string{"quantity_change", "unit_cost", "total_value"},
				"dimensions": []string{"dim_date", "dim_product", "dim_location"},
			},
			{
				"name": "fact_customer_behavior",
				"description": "顧客行為事實表 - 記錄顧客的各種行為事件",
				"measures": []string{"event_count", "session_duration", "page_views"},
				"dimensions": []string{"dim_date", "dim_customer", "dim_product", "dim_location"},
			},
		},
	}
}

// Phase2Runner Phase 2 執行器
type Phase2Runner struct {
	config    *config.Config
	db        *sql.DB
	reader    *Phase1ResultReader
	analyzer  *TableAnalysisOrchestrator
	mcpServer *mcp.MCPServer
}

// NewPhase2Runner 創建 Phase 2 執行器
func NewPhase2Runner(cfg *config.Config, db *sql.DB) *Phase2Runner {
	// 創建 phase1 結果讀取器
	reader := NewPhase1ResultReader("knowledge/phase1_analysis.json")

	// 創建 MCP 服務器
	mcpServer := mcp.NewMCPServer(db)

	// 創建表格分析協調器
	analyzer := NewTableAnalysisOrchestrator(cfg, reader, mcpServer)

	return &Phase2Runner{
		config:    cfg,
		db:        db,
		reader:    reader,
		analyzer:  analyzer,
		mcpServer: mcpServer,
	}
}

// Run 執行 Phase 2 AI 分析
func (p *Phase2Runner) Run() error {
	log.Println("=== Starting Phase 2: AI Analysis ===")

	// 檢查 LLM 配置
	log.Printf("LLM Configuration:")
	log.Printf("  Provider: %s", p.config.LLM.Provider)
	log.Printf("  Model: %s", p.config.LLM.Model)
	log.Printf("  Host: %s", p.config.LLM.Host)
	log.Printf("  Port: %d", p.config.LLM.Port)
	log.Printf("  Base URL: %s", p.config.LLM.BaseURL)

	// 初始化分析任務
	if err := p.analyzer.InitializeTasks(); err != nil {
		return fmt.Errorf("failed to initialize analysis tasks: %v", err)
	}

	// 執行分析
	ctx := context.Background()
	if err := p.runAnalysis(ctx); err != nil {
		return fmt.Errorf("failed to run analysis: %v", err)
	}

	// 保存結果
	if err := p.saveResults(); err != nil {
		return fmt.Errorf("failed to save results: %v", err)
	}

	log.Printf("Phase 2 AI analysis completed successfully")
	return nil
}

// runAnalysis 執行分析流程
func (p *Phase2Runner) runAnalysis(ctx context.Context) error {
	log.Println("Starting table analysis process...")

	for {
		// 獲取下一個任務
		task := p.analyzer.GetNextTask()
		if task == nil {
			// 所有任務都完成了
			break
		}

		log.Printf("Processing table: %s", task.TableName)

		// 開始任務
		p.analyzer.StartTask(task)

		// 分析表格
		result, err := p.analyzer.AnalyzeTable(ctx, task)
		if err != nil {
			log.Printf("Failed to analyze table %s: %v", task.TableName, err)
			p.analyzer.FailTask(task, err)
			continue
		}

		// 完成任務
		p.analyzer.CompleteTask(task, result)

		// 顯示進度
		progress := p.analyzer.GetProgress()
		log.Printf("Progress: %.1f%% (%d/%d completed)",
			progress["percentage"].(float64),
			progress["completed"].(int),
			progress["total"].(int))
	}

	log.Println("All table analyses completed")
	return nil
}

// saveResults 保存分析結果
func (p *Phase2Runner) saveResults() error {
	results := p.analyzer.GetResults()

	// 創建輸出結構
	output := map[string]interface{}{
		"phase":         "phase2",
		"description":   "AI-powered database analysis",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"llm_config": map[string]interface{}{
			"provider": p.config.LLM.Provider,
			"model":    p.config.LLM.Model,
			"host":     p.config.LLM.Host,
			"port":     p.config.LLM.Port,
		},
		"analysis_results": results,
		"summary":          p.generateSummary(results),
	}

	// 寫入商業邏輯分析結果
	if err := p.writeOutput(output, "knowledge/phase2_analysis.json"); err != nil {
		return err
	}

	// 生成並保存維度分析報告
	if err := p.generateDimensionAnalysis(results); err != nil {
		return fmt.Errorf("failed to generate dimension analysis: %v", err)
	}

	return nil
}

// generateSummary 生成總結
func (p *Phase2Runner) generateSummary(results map[string]*LLMAnalysisResult) map[string]interface{} {
	totalTables := len(results)
	totalRecommendations := 0
	totalIssues := 0
	totalInsights := 0

	for _, result := range results {
		totalRecommendations += len(result.Recommendations)
		totalIssues += len(result.Issues)
		totalInsights += len(result.Insights)
	}

	return map[string]interface{}{
		"total_tables_analyzed":             totalTables,
		"total_recommendations":             totalRecommendations,
		"total_issues_found":                totalIssues,
		"total_insights":                    totalInsights,
		"average_recommendations_per_table": float64(totalRecommendations) / float64(totalTables),
		"average_issues_per_table":          float64(totalIssues) / float64(totalTables),
		"average_insights_per_table":        float64(totalInsights) / float64(totalTables),
	}
}

// writeOutput 寫入輸出到文件
func (p *Phase2Runner) writeOutput(data interface{}, filename string) error {
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

	log.Printf("Phase 2 AI analysis results saved to %s", filename)
	return nil
}

// generateDimensionAnalysis 生成維度分析報告
func (p *Phase2Runner) generateDimensionAnalysis(tableResults map[string]*LLMAnalysisResult) error {
	log.Println("Generating dimension analysis report...")

	// 創建維度分析器
	dimensionAnalyzer := NewDimensionAnalyzer()

	// 分析所有表格並識別維度
	dimensions := dimensionAnalyzer.AnalyzeDimensions(tableResults)

	// 創建維度分析報告
	dimensionReport := map[string]interface{}{
		"phase":         "phase2_dimensions",
		"description":   "Database dimension analysis for BI and analytics",
		"database":      p.config.Database.DBName,
		"database_type": p.config.Database.Type,
		"timestamp":     time.Now(),
		"dimensions":    dimensions,
		"summary":       dimensionAnalyzer.GenerateSummary(dimensions),
	}

	// 寫入維度分析報告
	return p.writeOutput(dimensionReport, "knowledge/phase2_dimensions.json")
}
