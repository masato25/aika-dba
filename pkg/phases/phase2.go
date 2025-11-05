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

	// 生成 Phase 3 準備文件
	if err := p.generatePrePhase3Summary(results); err != nil {
		return fmt.Errorf("failed to generate pre-phase3 summary: %v", err)
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

// generatePrePhase3Summary 生成 Phase 3 準備總結
func (p *Phase2Runner) generatePrePhase3Summary(results map[string]*LLMAnalysisResult) error {
	log.Println("Generating pre-Phase 3 summary...")

	// 分析整體商業邏輯
	businessSummary := p.analyzeBusinessLogic(results)

	// 生成 Phase 3 建議
	phase3Suggestions := p.generatePhase3Suggestions(results)

	// 創建預 Phase 3 文件
	prePhase3Data := map[string]interface{}{
		"phase":              "pre_phase3",
		"description":        "Business logic summary and Phase 3 preparation guide",
		"database":           p.config.Database.DBName,
		"database_type":      p.config.Database.Type,
		"timestamp":          time.Now(),
		"business_summary":   businessSummary,
		"phase3_suggestions": phase3Suggestions,
		"custom_notes": map[string]interface{}{
			"business_domain":         "",
			"key_entities":            []string{},
			"important_relationships": []string{},
			"analysis_focus_areas":    []string{},
			"custom_dimensions":       []map[string]interface{}{},
			"additional_notes":        "",
		},
		"instructions": map[string]string{
			"business_domain":         "請描述這個資料庫的主要商業領域（例如：電商、物流、財務等）",
			"key_entities":            "列出系統中的關鍵實體（例如：顧客、訂單、商品等）",
			"important_relationships": "描述重要的業務關係（例如：顧客下訂單、商品屬於分類等）",
			"analysis_focus_areas":    "列出分析重點領域（例如：銷售分析、顧客行為、庫存管理等）",
			"custom_dimensions":       "如果需要額外的維度表，請在此描述",
			"additional_notes":        "其他需要記錄的重要信息或考慮點",
		},
	}

	// 寫入預 Phase 3 文件
	return p.writeOutput(prePhase3Data, "knowledge/pre_phase3_summary.json")
}

// analyzeBusinessLogic 分析整體商業邏輯
func (p *Phase2Runner) analyzeBusinessLogic(results map[string]*LLMAnalysisResult) map[string]interface{} {
	// 統計不同類型的表格
	tableCategories := map[string][]string{
		"customer_management":  {},
		"product_management":   {},
		"order_management":     {},
		"inventory_management": {},
		"marketing_promotion":  {},
		"content_reviews":      {},
		"shipping_logistics":   {},
		"other":                {},
	}

	// 根據表格名稱和分析內容分類
	for tableName, result := range results {
		category := p.categorizeTable(tableName, result)
		tableCategories[category] = append(tableCategories[category], tableName)
	}

	// 生成商業邏輯總結
	businessLogic := map[string]interface{}{
		"overall_business_domain": p.inferBusinessDomain(tableCategories),
		"core_business_processes": p.identifyCoreProcesses(tableCategories),
		"key_business_entities":   p.extractKeyEntities(tableCategories),
		"business_relationships":  p.analyzeRelationships(results),
		"table_categories":        tableCategories,
	}

	return businessLogic
}

// categorizeTable 將表格分類
func (p *Phase2Runner) categorizeTable(tableName string, result *LLMAnalysisResult) string {
	analysis := strings.ToLower(result.Analysis)

	// 根據表格名稱和分析內容進行分類
	switch {
	case strings.Contains(tableName, "customer") || strings.Contains(analysis, "顧客") || strings.Contains(analysis, "客戶"):
		return "customer_management"
	case strings.Contains(tableName, "product") || strings.Contains(tableName, "categories") || strings.Contains(analysis, "商品"):
		return "product_management"
	case strings.Contains(tableName, "order") || strings.Contains(tableName, "cart") || strings.Contains(analysis, "訂單"):
		return "order_management"
	case strings.Contains(tableName, "inventory") || strings.Contains(analysis, "庫存"):
		return "inventory_management"
	case strings.Contains(tableName, "coupon") || strings.Contains(tableName, "promo") || strings.Contains(analysis, "優惠") || strings.Contains(analysis, "促銷"):
		return "marketing_promotion"
	case strings.Contains(tableName, "review") || strings.Contains(analysis, "評論"):
		return "content_reviews"
	case strings.Contains(tableName, "shipment") || strings.Contains(tableName, "shipping") || strings.Contains(analysis, "運送"):
		return "shipping_logistics"
	default:
		return "other"
	}
}

// inferBusinessDomain 推斷商業領域
func (p *Phase2Runner) inferBusinessDomain(categories map[string][]string) string {
	// 根據表格分類推斷主要商業領域
	hasOrders := len(categories["order_management"]) > 0
	hasProducts := len(categories["product_management"]) > 0
	hasCustomers := len(categories["customer_management"]) > 0
	hasShipping := len(categories["shipping_logistics"]) > 0

	if hasOrders && hasProducts && hasCustomers {
		if hasShipping {
			return "電商平台 (E-commerce Platform)"
		}
		return "零售/銷售系統 (Retail/Sales System)"
	}

	if len(categories["inventory_management"]) > 0 && hasProducts {
		return "庫存管理系統 (Inventory Management System)"
	}

	return "通用商業系統 (General Business System)"
}

// identifyCoreProcesses 識別核心業務流程
func (p *Phase2Runner) identifyCoreProcesses(categories map[string][]string) []string {
	processes := []string{}

	if len(categories["customer_management"]) > 0 {
		processes = append(processes, "顧客管理 (Customer Management)")
	}

	if len(categories["product_management"]) > 0 {
		processes = append(processes, "商品目錄管理 (Product Catalog Management)")
	}

	if len(categories["order_management"]) > 0 {
		processes = append(processes, "訂單處理 (Order Processing)")
		processes = append(processes, "購物車管理 (Shopping Cart Management)")
	}

	if len(categories["inventory_management"]) > 0 {
		processes = append(processes, "庫存管理 (Inventory Management)")
	}

	if len(categories["marketing_promotion"]) > 0 {
		processes = append(processes, "行銷活動管理 (Marketing Campaign Management)")
	}

	if len(categories["content_reviews"]) > 0 {
		processes = append(processes, "用戶評論管理 (User Review Management)")
	}

	if len(categories["shipping_logistics"]) > 0 {
		processes = append(processes, "物流運送管理 (Shipping & Logistics Management)")
	}

	return processes
}

// extractKeyEntities 提取關鍵實體
func (p *Phase2Runner) extractKeyEntities(categories map[string][]string) []string {
	entities := []string{}

	for category, tables := range categories {
		if len(tables) > 0 {
			switch category {
			case "customer_management":
				entities = append(entities, "顧客 (Customer)")
			case "product_management":
				entities = append(entities, "商品 (Product)")
				entities = append(entities, "商品分類 (Category)")
			case "order_management":
				entities = append(entities, "訂單 (Order)")
				entities = append(entities, "訂單項目 (Order Item)")
			case "inventory_management":
				entities = append(entities, "庫存交易 (Inventory Transaction)")
			case "marketing_promotion":
				entities = append(entities, "優惠券 (Coupon)")
			case "content_reviews":
				entities = append(entities, "評論 (Review)")
			case "shipping_logistics":
				entities = append(entities, "運送 (Shipment)")
			}
		}
	}

	return entities
}

// analyzeRelationships 分析業務關係
func (p *Phase2Runner) analyzeRelationships(results map[string]*LLMAnalysisResult) []string {
	relationships := []string{}

	// 基於表格分析內容提取關係
	for _, result := range results {
		analysis := strings.ToLower(result.Analysis)

		if strings.Contains(analysis, "訂單") && strings.Contains(analysis, "顧客") {
			relationships = append(relationships, "顧客下訂單 (Customer places Orders)")
		}

		if strings.Contains(analysis, "商品") && strings.Contains(analysis, "訂單") {
			relationships = append(relationships, "訂單包含商品 (Orders contain Products)")
		}

		if strings.Contains(analysis, "商品") && strings.Contains(analysis, "分類") {
			relationships = append(relationships, "商品屬於分類 (Products belong to Categories)")
		}

		if strings.Contains(analysis, "評論") && strings.Contains(analysis, "商品") {
			relationships = append(relationships, "顧客評論商品 (Customers review Products)")
		}

		if strings.Contains(analysis, "運送") && strings.Contains(analysis, "訂單") {
			relationships = append(relationships, "訂單需要運送 (Orders require Shipping)")
		}
	}

	// 去重
	uniqueRelationships := make(map[string]bool)
	result := []string{}
	for _, rel := range relationships {
		if !uniqueRelationships[rel] {
			uniqueRelationships[rel] = true
			result = append(result, rel)
		}
	}

	return result
}

// generatePhase3Suggestions 生成 Phase 3 建議
func (p *Phase2Runner) generatePhase3Suggestions(results map[string]*LLMAnalysisResult) map[string]interface{} {
	return map[string]interface{}{
		"recommended_dimensions": []map[string]interface{}{
			{
				"name":            "dim_customer",
				"purpose":         "顧客維度 - 用於顧客行為分析和分群",
				"based_on_tables": []string{"customers", "customer_addresses"},
			},
			{
				"name":            "dim_product",
				"purpose":         "商品維度 - 用於商品銷售分析和分類統計",
				"based_on_tables": []string{"products", "categories", "product_variants"},
			},
			{
				"name":            "dim_date",
				"purpose":         "時間維度 - 用於時間序列分析和趨勢分析",
				"based_on_tables": []string{"orders", "customers", "reviews"},
			},
			{
				"name":            "dim_location",
				"purpose":         "地理維度 - 用於地區銷售分析和物流優化",
				"based_on_tables": []string{"customer_addresses", "shipments"},
			},
			{
				"name":            "dim_promotion",
				"purpose":         "行銷維度 - 用於促銷效果分析和ROI計算",
				"based_on_tables": []string{"coupons", "orders"},
			},
		},
		"recommended_fact_tables": []map[string]interface{}{
			{
				"name":         "fact_sales",
				"purpose":      "銷售事實表 - 記錄所有銷售交易的詳細信息",
				"grain":        "每筆訂單項目 (Per Order Item)",
				"key_measures": []string{"銷售金額", "數量", "折扣金額", "利潤"},
			},
			{
				"name":         "fact_inventory",
				"purpose":      "庫存事實表 - 記錄庫存變動和商品流轉",
				"grain":        "每次庫存變動 (Per Inventory Change)",
				"key_measures": []string{"變動數量", "單位成本", "總價值"},
			},
			{
				"name":         "fact_customer_behavior",
				"purpose":      "顧客行為事實表 - 記錄顧客的各種互動行為",
				"grain":        "每次顧客行為事件 (Per Customer Action)",
				"key_measures": []string{"事件計數", "停留時間", "頁面瀏覽數"},
			},
		},
		"analysis_focus_areas": []string{
			"銷售業績分析 (Sales Performance Analysis)",
			"顧客行為分析 (Customer Behavior Analysis)",
			"商品表現分析 (Product Performance Analysis)",
			"庫存優化分析 (Inventory Optimization Analysis)",
			"行銷效果分析 (Marketing Effectiveness Analysis)",
			"物流效率分析 (Logistics Efficiency Analysis)",
		},
		"implementation_notes": []string{
			"確保維度表設計符合星形模式或雪花模式",
			"考慮緩慢變化的維度 (Slowly Changing Dimensions)",
			"設計適當的彙總表以提升查詢性能",
			"考慮資料倉儲的最佳實踐和標準",
		},
	}
}
