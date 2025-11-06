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

	// 創建 Gin 引擎
	router := gin.Default()

	server := &APIServer{
		router:      router,
		db:          db,
		dbType:      dbType,
		config:      cfg,
		llmClient:   llmClient,
		vectorStore: vectorStore,
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

		// Phase 相關 API
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
	var err error
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

	if err != nil {
		c.JSON(500, map[string]string{"error": err.Error()})
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
	runner, err := phases.NewPhase1PostRunner(cfg)
	if err != nil {
		log.Fatalf("Failed to create Phase 1 post runner: %v", err)
	}

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 1 post-processing failed: %v", err)
	}
}

// runPhase1Put 執行 Phase 1 Put: 根據 post 分析結果更新 phase1
func runPhase1Put(cfg *config.Config) {
	runner, err := phases.NewPhase1PutRunner(cfg)
	if err != nil {
		log.Fatalf("Failed to create Phase 1 put runner: %v", err)
	}

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 1 put failed: %v", err)
	}
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
