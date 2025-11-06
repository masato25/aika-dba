package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/phases"
	"github.com/masato25/aika-dba/pkg/progress"
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
	progressMgr *progress.ProgressManager
	analyzer    *analyzer.DatabaseAnalyzer
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

	// 創建數據庫分析器
	dbAnalyzer := analyzer.NewDatabaseAnalyzer(db)

	// 創建 Gin 引擎
	router := gin.Default()

	server := &APIServer{
		router:      router,
		db:          db,
		dbType:      dbType,
		config:      cfg,
		llmClient:   llmClient,
		vectorStore: vectorStore,
		progressMgr: progress.NewProgressManager(),
		analyzer:    dbAnalyzer,
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
		api.GET("/phases/progress/:phase", s.handlePhaseProgress)
		api.GET("/phases/progress", s.handleAllProgress)
		api.GET("/phases/logs/:phase", s.handlePhaseLogs)

		// 向量數據庫 API
		api.GET("/vector/stats", s.handleVectorStats)
		api.GET("/vector/search", s.handleVectorSearch)
		api.GET("/vector/knowledge/:phase", s.handleVectorKnowledge)

		// 知識文件瀏覽
		api.GET("/knowledge/files", s.handleKnowledgeFiles)
		api.GET("/knowledge/files/:name", s.handleKnowledgeFile)

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

	// 檢查是否已經在運行
	if progress, exists := s.progressMgr.GetProgress(phase); exists && progress.Status == "running" {
		c.JSON(409, map[string]string{"error": "Phase " + phase + " is already running"})
		return
	}

	// 根據 phase 執行相應的操作
	go func() {
		var err error
		switch phase {
		case "phase1":
			err = s.runPhase1()
		case "phase1_post":
			err = s.runPhase1Post()
		case "phase1_put":
			err = s.runPhase1Put()
		case "phase2_prefix":
			err = s.runPhase2Prefix()
		case "phase2":
			err = s.runPhase2()
		case "phase3":
			err = s.runPhase3()
		default:
			s.progressMgr.FailPhase(phase, fmt.Errorf("unknown phase: %s", phase))
			return
		}

		if err != nil {
			s.progressMgr.FailPhase(phase, err)
		} else {
			s.progressMgr.CompletePhase(phase)
		}
	}()

	c.JSON(202, map[string]string{"message": "Phase " + phase + " started successfully"})
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

// handlePhaseProgress 處理獲取指定 phase 進度的請求
func (s *APIServer) handlePhaseProgress(c *gin.Context) {
	phase := c.Param("phase")

	progress, exists := s.progressMgr.GetProgress(phase)
	if !exists {
		c.JSON(404, map[string]string{"error": "Phase not found"})
		return
	}

	c.JSON(200, progress)
}

// handleAllProgress 處理獲取所有 phase 進度的請求
func (s *APIServer) handleAllProgress(c *gin.Context) {
	allProgress := s.progressMgr.GetAllProgress()
	c.JSON(200, allProgress)
}

// handlePhaseLogs 處理獲取指定 phase 日誌的請求
func (s *APIServer) handlePhaseLogs(c *gin.Context) {
	phase := c.Param("phase")

	progress, exists := s.progressMgr.GetProgress(phase)
	if !exists {
		c.JSON(404, map[string]string{"error": "Phase not found"})
		return
	}

	// 返回按時間降序排列的日誌
	c.JSON(200, progress.Logs)
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

// handleKnowledgeFiles 列出知識文件
func (s *APIServer) handleKnowledgeFiles(c *gin.Context) {
	const knowledgeDir = "knowledge"

	entries, err := os.ReadDir(knowledgeDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, []interface{}{})
			return
		}
		c.JSON(500, map[string]string{"error": err.Error()})
		return
	}

	type fileInfo struct {
		Name     string    `json:"name"`
		Size     int64     `json:"size_bytes"`
		ModTime  time.Time `json:"mod_time"`
		FullName string    `json:"-"`
	}

	files := make([]fileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			log.Printf("Warning: failed to read knowledge file info %s: %v", entry.Name(), err)
			continue
		}

		files = append(files, fileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	c.JSON(200, files)
}

// handleKnowledgeFile 讀取特定知識文件內容
func (s *APIServer) handleKnowledgeFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(400, map[string]string{"error": "File name is required"})
		return
	}

	name = filepath.Base(name)
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		c.JSON(400, map[string]string{"error": "Invalid file name"})
		return
	}

	knowledgeDir := "knowledge"
	baseDir, err := filepath.Abs(knowledgeDir)
	if err != nil {
		c.JSON(500, map[string]string{"error": "Failed to resolve knowledge directory"})
		return
	}

	filePath := filepath.Join(knowledgeDir, name)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		c.JSON(500, map[string]string{"error": "Failed to resolve file path"})
		return
	}

	if !strings.HasPrefix(absPath, baseDir) {
		c.JSON(400, map[string]string{"error": "Invalid file path"})
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, map[string]string{"error": "File not found"})
			return
		}
		c.JSON(500, map[string]string{"error": err.Error()})
		return
	}

	response := map[string]interface{}{
		"name":       name,
		"size_bytes": len(data),
		"path":       name,
	}

	if info, err := os.Stat(absPath); err == nil {
		response["mod_time"] = info.ModTime()
	}

	var parsed interface{}
	if strings.HasSuffix(strings.ToLower(name), ".json") {
		if err := json.Unmarshal(data, &parsed); err == nil {
			response["type"] = "json"
			response["content"] = parsed
		} else {
			response["type"] = "text"
			response["content"] = string(data)
			response["parse_error"] = fmt.Sprintf("failed to parse JSON: %v", err)
		}
	} else {
		response["type"] = "text"
		response["content"] = string(data)
	}

	c.JSON(200, response)
}

// writeOutput 寫入輸出到文件
func (s *APIServer) writeOutput(data interface{}, filename string) error {
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

	return nil
}

// RunServer 啟動 HTTP 服務器
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
func (s *APIServer) runPhase1() error {
	phase := "phase1"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	// 創建 phase 日誌器
	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	// 開始進度追蹤
	tables, err := s.analyzer.GetAllTables()
	if err != nil {
		return err
	}

	totalTables := len(tables)
	s.progressMgr.StartPhase(phase, totalTables)

	logger.Info(fmt.Sprintf("Starting Phase 1: Statistical Analysis - Found %d tables", totalTables))

	// 分析每個表格
	tableAnalyses := make(map[string]interface{})
	for i, tableName := range tables {
		logger.Info(fmt.Sprintf("Analyzing table %d/%d: %s", i+1, totalTables, tableName))

		// 使用分析器的 AnalyzeTable 方法
		analysis, err := s.analyzer.AnalyzeTable(tableName, s.config.Schema.MaxSamples)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to analyze table %s: %v", tableName, err))
			continue
		}

		tableAnalyses[tableName] = analysis

		// 更新進度
		s.progressMgr.UpdateProgress(phase, i+1, fmt.Sprintf("Analyzed table: %s", tableName))
		logger.Debug(fmt.Sprintf("Completed analysis of table: %s", tableName))
	}

	// 創建輸出
	output := map[string]interface{}{
		"database":      s.config.Database.DBName,
		"database_type": s.config.Database.Type,
		"timestamp":     time.Now(),
		"tables_count":  len(tables),
		"tables":        tableAnalyses,
	}

	// 寫入文件
	if err := s.writeOutput(output, "knowledge/phase1_analysis.json"); err != nil {
		return err
	}

	// 將知識存儲到向量數據庫
	if err := s.vectorStore.StorePhaseKnowledge("phase1", output); err != nil {
		logger.Warn(fmt.Sprintf("Failed to store phase1 knowledge in vector store: %v", err))
		// 不返回錯誤，因為 JSON 文件已經寫入成功
	} else {
		logger.Info("Phase 1 knowledge stored in vector database")
	}

	logger.Info("Phase 1 completed. Results saved to knowledge/phase1_analysis.json")
	return nil
}

// runPhase1Post 執行 Phase 1 後置處理
func (s *APIServer) runPhase1Post() error {
	phase := "phase1_post"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	s.progressMgr.StartPhase(phase, 4)
	statusMessage := "Checking existing post-processing responses"
	s.progressMgr.UpdateProgress(phase, 1, statusMessage)
	s.progressMgr.AddLog(phase, "info", "Starting Phase 1 post-processing workflow")
	logger.Info("Starting Phase 1 Post-Processing")

	responseFile := "knowledge/phase1_post_responses.json"
	if _, err := os.Stat(responseFile); err == nil {
		s.progressMgr.UpdateProgress(phase, 2, "Applying user responses to post-processing workflow")
		s.progressMgr.AddLog(phase, "info", "Found phase1_post_responses.json, applying decisions")
	} else if os.IsNotExist(err) {
		s.progressMgr.UpdateProgress(phase, 2, "Generating review questions for Phase 1 results")
		s.progressMgr.AddLog(phase, "info", "No user responses found, generating questions for review")
	} else {
		s.progressMgr.AddLog(phase, "warn", fmt.Sprintf("Unable to check response file: %v", err))
	}

	s.progressMgr.UpdateProgress(phase, 3, "Running Phase 1 post-processing")

	runner, err := phases.NewPhase1PostRunner(s.config)
	if err != nil {
		return fmt.Errorf("failed to create Phase 1 post runner: %w", err)
	}

	if err := runner.Run(); err != nil {
		return fmt.Errorf("Phase 1 post-processing failed: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 4, "Phase 1 post-processing completed")
	s.progressMgr.AddLog(phase, "info", "Phase 1 post-processing completed successfully")
	logger.Info("Phase 1 post-processing completed successfully")
	return nil
}

// runPhase1Put 執行 Phase 1 Put
func (s *APIServer) runPhase1Put() error {
	phase := "phase1_put"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	s.progressMgr.StartPhase(phase, 4)
	s.progressMgr.UpdateProgress(phase, 1, "Validating prerequisites for Phase 1 put")
	s.progressMgr.AddLog(phase, "info", "Starting Phase 1 put workflow")
	logger.Info("Starting Phase 1 Put: updating Phase 1 analysis with post decisions")

	requiredFiles := []string{
		"knowledge/phase1_analysis.json",
		"knowledge/phase1_post_analysis.json",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("required knowledge file missing: %s", file)
			}
			return fmt.Errorf("failed to access %s: %w", file, err)
		}
	}

	s.progressMgr.UpdateProgress(phase, 2, "Preparing Phase 1 put runner")

	runner, err := phases.NewPhase1PutRunner(s.config)
	if err != nil {
		return fmt.Errorf("failed to create Phase 1 put runner: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 3, "Applying post-processing decisions to Phase 1 analysis")

	if err := runner.Run(); err != nil {
		return fmt.Errorf("Phase 1 put failed: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 4, "Phase 1 put completed successfully")
	s.progressMgr.AddLog(phase, "info", "Phase 1 analysis updated with Phase 1 post decisions")
	logger.Info("Phase 1 Put completed successfully")
	return nil
}

// runPhase2Prefix 執行 Phase 2 前置處理: 欄位深度分析
func (s *APIServer) runPhase2Prefix() error {
	phase := "phase2_prefix"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	// 創建 phase 日誌器
	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	// 開始進度追蹤 - 估計步驟數
	s.progressMgr.StartPhase(phase, 10) // 估計 10 個步驟

	logger.Info("Starting Phase 2 Prefix: Column Depth Analysis")

	log.Println("DEBUG: Creating Phase 2 prefix runner...")
	runner, err := phases.NewPhase2PrefixRunner(s.config)
	if err != nil {
		return fmt.Errorf("failed to create Phase 2 prefix runner: %w", err)
	}
	log.Println("DEBUG: Phase 2 prefix runner created successfully")

	s.progressMgr.UpdateProgress(phase, 1, "Phase 2 prefix runner created")

	log.Println("DEBUG: Calling runner.Run()...")
	if err := runner.Run(); err != nil {
		return fmt.Errorf("Phase 2 prefix failed: %w", err)
	}
	log.Println("DEBUG: Phase 2 prefix completed successfully")

	s.progressMgr.UpdateProgress(phase, 10, "Phase 2 prefix completed")
	logger.Info("Phase 2 prefix completed successfully")
	return nil
}

// runPhase2 執行 Phase 2: AI 分析
func (s *APIServer) runPhase2() error {
	phase := "phase2"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	// 創建 phase 日誌器
	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	// 開始進度追蹤 - 估計步驟數
	s.progressMgr.StartPhase(phase, 10) // 估計 10 個步驟

	logger.Info("Starting Phase 2: AI Business Logic Analysis")

	runner, err := phases.NewPhase2Runner(s.config, s.db)
	if err != nil {
		return fmt.Errorf("failed to create Phase 2 runner: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 1, "Phase 2 runner created")

	if err := runner.Run(); err != nil {
		return fmt.Errorf("Phase 2 failed: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 10, "Phase 2 completed")
	logger.Info("Phase 2 completed successfully")
	return nil
}

// runPhase3 執行 Phase 3: 商業邏輯描述生成
func (s *APIServer) runPhase3() error {
	phase := "phase3"
	debugEnabled := strings.ToLower(s.config.Logging.Level) == "debug"

	// 創建 phase 日誌器
	logger := progress.NewPhaseLogger(phase, s.progressMgr, debugEnabled)

	// 開始進度追蹤 - 估計步驟數
	s.progressMgr.StartPhase(phase, 10) // 估計 10 個步驟

	logger.Info("Starting Phase 3: Business Logic Description Generation")

	// 創建 LLM 客戶端
	llmClient := llm.NewClient(s.config)

	s.progressMgr.UpdateProgress(phase, 1, "LLM client created")

	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(s.config)
	if err != nil {
		return fmt.Errorf("failed to create knowledge manager: %w", err)
	}
	defer knowledgeMgr.Close()

	s.progressMgr.UpdateProgress(phase, 2, "Knowledge manager created")

	// 創建 Phase 3 執行器
	runner := phases.NewPhase3Runner(s.config, llmClient, knowledgeMgr)

	s.progressMgr.UpdateProgress(phase, 3, "Phase 3 runner created")

	if err := runner.Run(context.Background()); err != nil {
		return fmt.Errorf("Phase 3 failed: %w", err)
	}

	s.progressMgr.UpdateProgress(phase, 10, "Phase 3 completed")
	logger.Info("Phase 3 completed successfully")
	return nil
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
