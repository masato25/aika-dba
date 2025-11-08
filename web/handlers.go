package web

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/phases"
	"github.com/masato25/aika-dba/pkg/preparer"
	"github.com/masato25/aika-dba/pkg/query"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// APIServer API 服務器
type APIServer struct {
	router      *gin.Engine
	db          *sql.DB
	dbType      string
	config      *config.Config
	llmClient   *llm.Client
	vectorStore *vectorstore.KnowledgeManager
	preparer    *preparer.KnowledgePreparer
	queryIntf   *query.QueryInterface
	logger      *log.Logger
}

// NewAPIServer 創建 API 服務器
func NewAPIServer(db *sql.DB, dbType string, cfg *config.Config) (*APIServer, error) {
	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)

	// 創建知識管理器
	vectorStore, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	// 創建 logger
	logger := log.New(os.Stdout, "[WebAPI] ", log.LstdFlags)

	// 創建知識庫準備器
	preparer := preparer.NewKnowledgePreparer(db, vectorStore, logger)

	// 創建查詢接口
	queryIntf := query.NewQueryInterface(vectorStore, llmClient, db, logger)

	// 創建 Gin 引擎
	router := gin.Default()

	server := &APIServer{
		router:      router,
		db:          db,
		dbType:      dbType,
		config:      cfg,
		llmClient:   llmClient,
		vectorStore: vectorStore,
		preparer:    preparer,
		queryIntf:   queryIntf,
		logger:      logger,
	}

	server.setupRoutes()
	return server, nil
}

// setupRoutes 設定路由
func (s *APIServer) setupRoutes() {
	// 靜態文件服務
	s.router.Static("/static", "web/static")

	// 前端頁面
	s.router.GET("/", s.handleIndex)

	// API 路由
	api := s.router.Group("/api")
	{
		// 健康檢查
		api.GET("/health", s.handleHealth)

		// 知識庫管理
		api.POST("/knowledge/prepare", s.handlePrepareKnowledge)
		api.POST("/knowledge/query", s.handleQuery)
		api.POST("/knowledge/suggest", s.handleQuerySuggestion)

		// Phase 相關 API (保留向後相容性)
		api.POST("/phases/trigger/:phase", s.handleTriggerPhase)
		api.GET("/phases/status", s.handlePhaseStatus)

		// 向量數據庫 API
		api.GET("/vector/stats", s.handleVectorStats)
		api.GET("/vector/search", s.handleVectorSearch)
		api.GET("/vector/knowledge/:phase", s.handleVectorKnowledge)

		// 資料庫總覽
		api.GET("/database/overview", s.handleDatabaseOverview)
	}
}

// Start 啟動服務器
func (s *APIServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	return s.router.Run(addr)
}

// handleIndex 處理首頁請求
func (s *APIServer) handleIndex(c *gin.Context) {
	htmlContent, err := os.ReadFile("web/index.html")
	if err != nil {
		log.Printf("Error reading HTML file: %v", err)
		c.String(500, "Internal Server Error")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, string(htmlContent))
}

// handleHealth 健康檢查
func (s *APIServer) handleHealth(c *gin.Context) {
	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now(),
	}
	c.JSON(200, response)
}

// handlePrepareKnowledge 處理知識庫準備請求
func (s *APIServer) handlePrepareKnowledge(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Printf("Panic in handlePrepareKnowledge: %v", r)
			c.JSON(500, map[string]interface{}{
				"success": false,
				"error":   "Internal server error",
			})
		}
	}()

	s.logger.Println("開始準備知識庫...")

	// 執行知識庫準備
	err := s.preparer.PrepareKnowledge()
	if err != nil {
		s.logger.Printf("知識庫準備失敗: %v", err)
		// 記錄錯誤的詳細信息以調試
		s.logger.Printf("錯誤類型: %T", err)
		s.logger.Printf("錯誤字符串: %q", err.Error())

		// 確保錯誤消息是有效的 JSON 字符串
		errorMsg := strings.ReplaceAll(err.Error(), "\n", " ")
		errorMsg = strings.ReplaceAll(errorMsg, "\r", " ")
		errorMsg = strings.ReplaceAll(errorMsg, "\t", " ")

		c.JSON(500, map[string]interface{}{
			"success": false,
			"error":   errorMsg,
		})
		return
	}

	s.logger.Println("知識庫準備完成")
	c.JSON(200, map[string]interface{}{
		"success": true,
		"message": "知識庫準備完成",
	})
}

// handleQuery 處理自然語言查詢請求
func (s *APIServer) handleQuery(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, map[string]interface{}{
			"success": false,
			"error":   "請提供問題內容",
		})
		return
	}

	s.logger.Printf("處理查詢: %s", req.Question)

	// 使用 MarketingQueryRunner 處理營銷查詢
	runner := phases.NewMarketingQueryRunner(s.config, s.db)
	result, err := runner.ExecuteMarketingQuery(req.Question)
	if err != nil {
		s.logger.Printf("營銷查詢失敗: %v", err)
		c.JSON(500, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 格式化回應
	response := map[string]interface{}{
		"success": true,
		"query":   result.Query,
		"sql":     result.SQLQuery,
		"results": result.Results,
		"count":   len(result.Results),
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	if result.Explanation != "" {
		response["explanation"] = result.Explanation
	}

	if result.BusinessInsights != "" {
		response["insights"] = result.BusinessInsights
	}

	s.logger.Printf("查詢完成，返回 %d 個結果", len(result.Results))
	c.JSON(200, response)
}

// handleQuerySuggestion 處理查詢建議請求
func (s *APIServer) handleQuerySuggestion(c *gin.Context) {
	var req struct {
		PartialQuery string `json:"partial_query" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, map[string]interface{}{
			"success": false,
			"error":   "請提供部分查詢內容",
		})
		return
	}

	if len(req.PartialQuery) < 2 {
		c.JSON(200, map[string]interface{}{
			"success":     true,
			"suggestions": []string{},
		})
		return
	}

	s.logger.Printf("生成查詢建議: %s", req.PartialQuery)

	// 直接使用後備的自然語言建議，因為當前知識庫主要包含技術性內容
	s.logger.Printf("使用後備自然語言建議 - 輸入: %s", req.PartialQuery)
	suggestions := s.generateFallbackSuggestions(req.PartialQuery)
	s.logger.Printf("生成的建議: %v", suggestions)

	// 同時輸出到控制台以便調試
	fmt.Printf("DEBUG: 建議結果: %v\n", suggestions)

	c.JSON(200, map[string]interface{}{
		"success":     true,
		"suggestions": suggestions,
	})
}

// generateSuggestionsFromKnowledge 基於知識庫數據生成自然語言建議
func (s *APIServer) generateSuggestionsFromKnowledge(partialQuery string, knowledgeResults []vectorstore.KnowledgeResult) []string {
	if len(knowledgeResults) == 0 {
		return []string{}
	}

	suggestions := make([]string, 0, 3)
	seen := make(map[string]bool)

	// 分析知識內容，提取關鍵實體和概念
	entities := s.extractEntitiesFromKnowledge(knowledgeResults)

	// 根據輸入和實體生成建議
	switch strings.ToLower(partialQuery) {
	case "商品":
		if s.containsEntity(entities, "銷售") || s.containsEntity(entities, "銷量") {
			suggestions = append(suggestions, "給我過去一週商品銷售統計")
		}
		if s.containsEntity(entities, "價格") {
			suggestions = append(suggestions, "顯示商品價格區間分析")
		}
		if s.containsEntity(entities, "庫存") {
			suggestions = append(suggestions, "找出庫存不足的商品")
		}
		suggestions = append(suggestions, "顯示所有商品的基本信息")
		suggestions = append(suggestions, "統計不同類別的商品數量")

	case "銷售":
		suggestions = append(suggestions, "給我月度銷售趨勢分析")
		suggestions = append(suggestions, "找出銷售額最高的商品")
		suggestions = append(suggestions, "計算總銷售額和平均訂單價值")

	case "客戶":
		suggestions = append(suggestions, "顯示客戶消費習慣分析")
		suggestions = append(suggestions, "找出最有價值的客戶群體")
		suggestions = append(suggestions, "統計客戶地域分佈")

	case "訂單":
		suggestions = append(suggestions, "顯示最近的訂單狀態")
		suggestions = append(suggestions, "找出處理時間最長的訂單")
		suggestions = append(suggestions, "計算訂單完成率統計")

	default:
		// 基於知識內容生成通用建議
		for _, result := range knowledgeResults {
			content := strings.ToLower(result.Content)

			if strings.Contains(content, "統計") || strings.Contains(content, "分析") {
				suggestion := fmt.Sprintf("給我%s相關的統計數據", partialQuery)
				if !seen[suggestion] {
					suggestions = append(suggestions, suggestion)
					seen[suggestion] = true
				}
			}

			if strings.Contains(content, "排名") || strings.Contains(content, "top") {
				suggestion := fmt.Sprintf("顯示%s的排名情況", partialQuery)
				if !seen[suggestion] {
					suggestions = append(suggestions, suggestion)
					seen[suggestion] = true
				}
			}

			if strings.Contains(content, "趨勢") || strings.Contains(content, "變化") {
				suggestion := fmt.Sprintf("分析%s的趨勢變化", partialQuery)
				if !seen[suggestion] {
					suggestions = append(suggestions, suggestion)
					seen[suggestion] = true
				}
			}
		}
	}

	// 限制建議數量
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}

	// 如果沒有生成足夠的建議，添加通用建議
	for len(suggestions) < 3 {
		genericSuggestion := fmt.Sprintf("顯示所有%s相關的信息", partialQuery)
		if !seen[genericSuggestion] {
			suggestions = append(suggestions, genericSuggestion)
			seen[genericSuggestion] = true
		} else {
			break
		}
	}

	return suggestions
}

// extractEntitiesFromKnowledge 從知識結果中提取關鍵實體
func (s *APIServer) extractEntitiesFromKnowledge(knowledgeResults []vectorstore.KnowledgeResult) []string {
	entities := make([]string, 0)
	seen := make(map[string]bool)

	for _, result := range knowledgeResults {
		content := strings.ToLower(result.Content)

		// 提取可能的實體詞
		entityPatterns := []string{
			"銷售", "銷量", "營業額", "營收",
			"商品", "產品", "貨物",
			"客戶", "用戶", "買家",
			"訂單", "交易", "購買",
			"庫存", "存貨", "庫存量",
			"價格", "單價", "總價",
			"統計", "分析", "數據",
		}

		for _, pattern := range entityPatterns {
			if strings.Contains(content, pattern) && !seen[pattern] {
				entities = append(entities, pattern)
				seen[pattern] = true
			}
		}
	}

	return entities
}

// generateFallbackSuggestions 生成後備的自然語言建議
func (s *APIServer) generateFallbackSuggestions(partialQuery string) []string {
	// 根據輸入的關鍵詞提供相應的自然語言建議
	switch strings.ToLower(partialQuery) {
	case "商品":
		return []string{
			"顯示所有商品的清單",
			"找出最暢銷的商品",
			"計算商品的平均價格",
		}
	case "銷售":
		return []string{
			"顯示銷售數據統計",
			"找出銷售額最高的月份",
			"計算總銷售額",
		}
	case "客戶":
		return []string{
			"顯示客戶信息清單",
			"找出最活躍的客戶",
			"統計客戶總數",
		}
	case "訂單":
		return []string{
			"顯示最近的訂單",
			"計算訂單總金額",
			"找出待處理的訂單",
		}
	case "庫存":
		return []string{
			"檢查商品庫存狀況",
			"找出缺貨的商品",
			"計算庫存總價值",
		}
	default:
		// 通用建議
		return []string{
			fmt.Sprintf("顯示所有%s相關的數據", partialQuery),
			fmt.Sprintf("統計%s的數量", partialQuery),
			fmt.Sprintf("找出%s的詳細信息", partialQuery),
		}
	}
}

// isTechnicalContent 檢查知識內容是否為技術性的（包含數據庫schema、SQL等）
func (s *APIServer) isTechnicalContent(knowledgeResults []vectorstore.KnowledgeResult) bool {
	for _, result := range knowledgeResults {
		content := strings.ToLower(result.Content)

		// 檢查是否包含技術性關鍵詞
		technicalKeywords := []string{
			"schema", "table", "column", "index", "constraint",
			"primary key", "foreign key", "create table", "select",
			"insert", "update", "delete", "sql", "postgresql",
			"mysql", "database", "query", "join", "where",
		}

		for _, keyword := range technicalKeywords {
			if strings.Contains(content, keyword) {
				return true
			}
		}
	}
	return false
}

// containsEntity 檢查實體列表是否包含指定實體
func (s *APIServer) containsEntity(entities []string, entity string) bool {
	for _, e := range entities {
		if e == entity {
			return true
		}
	}
	return false
}

// handleDatabaseOverview 資料庫總覽
func (s *APIServer) handleDatabaseOverview(c *gin.Context) {
	response := map[string]interface{}{
		"message": "Database analysis functionality is not yet implemented",
		"status":  "coming_soon",
		"time":    time.Now(),
	}
	c.JSON(200, response)
}

// handleTriggerPhase 處理觸發 phase 的請求
func (s *APIServer) handleTriggerPhase(c *gin.Context) {
	phase := c.Param("phase")

	// 根據 phase 執行相應的操作
	switch phase {
	case "phase1":
		runPhase1(s.db, s.config)
	case "phase2_prefix":
		runPhase2Prefix(s.config)
	case "phase2":
		runPhase2(s.db, s.config)
	case "phase3":
		runPhase3(s.config)
	default:
		c.JSON(400, map[string]string{"error": "Unknown phase: " + phase})
		return
	}

	c.JSON(200, map[string]string{"message": "Phase " + phase + " executed successfully"})
}

// handlePhaseStatus 處理獲取 phase 狀態的請求
func (s *APIServer) handlePhaseStatus(c *gin.Context) {
	// 這裡可以實現更複雜的狀態追蹤邏輯
	// 目前返回簡單的狀態
	status := map[string]interface{}{
		"status":  "ready",
		"message": "System is ready to execute phases",
		"time":    time.Now(),
	}
	c.JSON(200, status)
}

// handleVectorStats 處理獲取向量數據庫統計的請求
func (s *APIServer) handleVectorStats(c *gin.Context) {
	stats, err := s.vectorStore.GetKnowledgeStats()
	if err != nil {
		c.JSON(500, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(200, stats)
}

// handleVectorSearch 處理向量搜索請求
func (s *APIServer) handleVectorSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(400, map[string]string{"error": "Query parameter 'q' is required"})
		return
	}

	// 搜索所有 phase 的知識
	results, err := s.vectorStore.RetrieveCrossPhaseKnowledge(query, []string{"phase1", "phase2", "phase3"}, 5)
	if err != nil {
		c.JSON(500, map[string]string{"error": err.Error()})
		return
	}

	// 格式化結果
	formattedResults := make([]map[string]interface{}, len(results))
	for i, result := range results {
		formattedResults[i] = map[string]interface{}{
			"content":  result.Content,
			"metadata": result.Metadata,
			"score":    result.Score,
		}
	}

	c.JSON(200, formattedResults)
}

// handleVectorKnowledge 處理獲取指定 phase 知識的請求
func (s *APIServer) handleVectorKnowledge(c *gin.Context) {
	phase := c.Param("phase")

	// 搜索該 phase 的知識
	results, err := s.vectorStore.RetrievePhaseKnowledge(phase, "phase:"+phase, 10)
	if err != nil {
		c.JSON(500, map[string]string{"error": err.Error()})
		return
	}

	// 格式化結果
	formattedResults := make([]map[string]interface{}, len(results))
	for i, result := range results {
		formattedResults[i] = map[string]interface{}{
			"content":  result.Content,
			"metadata": result.Metadata,
			"score":    result.Score,
		}
	}

	c.JSON(200, formattedResults)
}

// runServer 啟動 HTTP 服務器
func RunServer(db *sql.DB, cfg *config.Config) {
	// 建立 API 服務器
	server, err := NewAPIServer(db, cfg.Database.Type, cfg)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}

	// 啟動服務器
	log.Fatal(server.Start(cfg.App.Port))
}

// runPhase1 執行 Phase 1: 統計分析
func runPhase1(db *sql.DB, cfg *config.Config) {
	analyzer := analyzer.NewDatabaseAnalyzer(db)
	runner, err := phases.NewPhase1Runner(analyzer, cfg)
	if err != nil {
		log.Fatalf("Failed to create Phase 1 runner: %v", err)
	}

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 1 failed: %v", err)
	}
}

// runPhase1Post 執行 Phase 1 後置處理: 數據庫分析和清理
func runPhase1Post(cfg *config.Config) {
	// 舊的 Phase 1 Post 功能已被重構移除
	log.Println("Phase 1 Post functionality has been removed in the refactor")
}

// runPhase1Put 執行 Phase 1 Put: 根據 post 分析結果更新 phase1
func runPhase1Put(cfg *config.Config) {
	// 舊的 Phase 1 Put 功能已被重構移除
	log.Println("Phase 1 Put functionality has been removed in the refactor")
}

// runPhase2 執行 Phase 2: AI 分析
func runPhase2(db *sql.DB, cfg *config.Config) {
	runner, err := phases.NewPhase2Runner(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create Phase 2 runner: %v", err)
	}

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 2 failed: %v", err)
	}
}

// runPhase2Prefix 執行 Phase 2 前置處理: 欄位深度分析
func runPhase2Prefix(cfg *config.Config) {
	log.Println("DEBUG: Starting runPhase2Prefix function")

	log.Println("DEBUG: Creating Phase 2 prefix runner...")
	runner, err := phases.NewPhase2PrefixRunner(cfg)
	if err != nil {
		log.Fatalf("Failed to create Phase 2 prefix runner: %v", err)
	}
	log.Println("DEBUG: Phase 2 prefix runner created successfully")

	log.Println("DEBUG: Calling runner.Run()...")
	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 2 prefix failed: %v", err)
	}
	log.Println("DEBUG: Phase 2 prefix completed successfully")
}

// runPhase3 執行 Phase 3: 商業邏輯描述生成
func runPhase3(cfg *config.Config) {
	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)

	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create knowledge manager: %v", err)
	}
	defer knowledgeMgr.Close()

	// 創建 Phase 3 執行器
	runner := phases.NewPhase3Runner(cfg, llmClient, knowledgeMgr)

	if err := runner.Run(context.Background()); err != nil {
		log.Fatalf("Phase 3 failed: %v", err)
	}
}

// runMarketingQuery 執行營銷查詢
func runMarketingQuery(db *sql.DB, cfg *config.Config, query string) {
	if query == "" {
		log.Fatalf("Query parameter is required for marketing command. Use -query flag.")
	}

	log.Printf("Executing marketing query: %s", query)

	runner := phases.NewMarketingQueryRunner(cfg, db)

	result, err := runner.ExecuteMarketingQuery(query)
	if err != nil {
		log.Fatalf("Marketing query failed: %v", err)
	}

	// 輸出結果
	fmt.Println("\n=== Marketing Query Results ===")
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Timestamp: %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))

	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
		return
	}

	fmt.Printf("SQL Query: %s\n", result.SQLQuery)
	fmt.Printf("Explanation: %s\n", result.Explanation)
	fmt.Printf("Results: %d rows\n", len(result.Results))

	if len(result.Results) > 0 {
		fmt.Println("\nSample Results:")
		// 顯示前 5 行結果
		for i, row := range result.Results {
			if i >= 5 {
				break
			}
			fmt.Printf("Row %d: ", i+1)
			for key, value := range row {
				fmt.Printf("%s=%v ", key, value)
			}
			fmt.Println()
		}
	}

	if result.BusinessInsights != "" {
		fmt.Println("\nBusiness Insights:")
		fmt.Println(result.BusinessInsights)
	}

	// 保存查詢結果
	if err := runner.SaveQueryResult(result); err != nil {
		log.Printf("Warning: Failed to save query result: %v", err)
	} else {
		log.Println("Query result saved to vector store")
	}
}

// runDeleteVectorData 執行向量數據刪除
func runDeleteVectorData(cfg *config.Config, phasesStr string) {
	log.Printf("Starting vector data deletion for phases: %s", phasesStr)

	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create knowledge manager: %v", err)
	}
	defer knowledgeMgr.Close()

	// 解析要刪除的 phases
	phaseList := strings.Split(phasesStr, ",")
	for i, phase := range phaseList {
		phaseList[i] = strings.TrimSpace(phase)
	}

	// 刪除每個 phase 的向量數據
	for _, phase := range phaseList {
		log.Printf("Deleting vector data for phase: %s", phase)
		if err := knowledgeMgr.DeletePhaseKnowledge(phase); err != nil {
			log.Printf("Warning: Failed to delete phase %s knowledge: %v", phase, err)
		} else {
			log.Printf("Successfully deleted vector data for phase: %s", phase)
		}
	}

	// 顯示統計信息
	stats, err := knowledgeMgr.GetKnowledgeStats()
	if err != nil {
		log.Printf("Warning: Failed to get knowledge stats: %v", err)
	} else {
		log.Printf("Vector data deletion completed. Current stats: %+v", stats)
	}
}
