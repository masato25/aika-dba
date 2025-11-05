package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
	"github.com/masato25/aika-dba/internal/ai"
	"github.com/masato25/aika-dba/internal/analyzer"
	"github.com/masato25/aika-dba/internal/mcp"
	"github.com/masato25/aika-dba/pkg/config"
	"github.com/masato25/aika-dba/internal/schema"
)

// APIServer API 服務器
type APIServer struct {
	router        *mux.Router
	schemaReader  *schema.SchemaReader
	analyzer      *analyzer.DatabaseAnalyzer
	mcpServer     *mcp.MCPServer
	aiAnalyzer    *ai.AIAnalyzer // 可選，如果 LLM 不可用則為 nil
	knowledgeBase *ai.KnowledgeBase
}

// NewAPIServer 創建 API 服務器
func NewAPIServer(db *sql.DB, dbType string) (*APIServer, error) {
	// 建立架構讀取器
	schemaReader := schema.NewSchemaReader(db, dbType)

	// 建立資料分析器
	dataAnalyzer := analyzer.NewDatabaseAnalyzer(db, dbType)

	// 讀取資料庫架構
	dbSchema, err := schemaReader.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	// 建立 MCP 資料分析器
	mcpAnalyzer := mcp.NewDatabaseDataAnalyzer(db, dbType, dbSchema)

	// 建立 MCP Server
	mcpServer := mcp.NewMCPServer(dbSchema, mcpAnalyzer)

	// 建立 LLM 客戶端（可選，如果連接失敗會記錄警告但不阻止應用程式啟動）
	var llmClient *ai.OpenAICompatibleClient
	var aiAnalyzer *ai.AIAnalyzer

	llmClient = ai.NewOpenAICompatibleClient("http://localhost:8080/v1", "", "Qwen/Qwen2.5-Coder-3B-Instruct-GGUF:Q4_K_M")

	// 測試 LLM 連接（可選）
	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = llmClient.GenerateAnalysis(testCtx, "test", nil)
	if err != nil {
		log.Printf("Warning: LLM client connection test failed: %v", err)
		log.Println("AI analysis features will be limited. Please ensure LLM service is running.")
		// 不阻止應用程式啟動，只記錄警告
		aiAnalyzer = nil
	} else {
		// 建立 AI 分析器
		aiAnalyzer = ai.NewAIAnalyzer(mcpServer, llmClient)
	}

	// 建立知識庫
	knowledgeBase, err := ai.NewKnowledgeBase("knowledge")
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge base: %w", err)
	}

	server := &APIServer{
		router:        mux.NewRouter(),
		schemaReader:  schemaReader,
		analyzer:      dataAnalyzer,
		mcpServer:     mcpServer,
		aiAnalyzer:    aiAnalyzer, // 可為 nil
		knowledgeBase: knowledgeBase,
	}

	server.setupRoutes()
	return server, nil
}

// setupRoutes 設定路由
func (s *APIServer) setupRoutes() {
	// 健康檢查
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// 資料庫總覽
	s.router.HandleFunc("/api/database/overview", s.handleDatabaseOverview).Methods("GET")

	// 表格分析
	s.router.HandleFunc("/api/tables/{tableName}/analysis", s.handleTableAnalysis).Methods("GET")
	s.router.HandleFunc("/api/tables/{tableName}/ai-analysis", s.handleTableAIAnalysis).Methods("POST")

	// 知識庫
	s.router.HandleFunc("/api/knowledge", s.handleKnowledgeList).Methods("GET")
	s.router.HandleFunc("/api/knowledge/{tableName}", s.handleKnowledgeGet).Methods("GET")
	s.router.HandleFunc("/api/knowledge/search", s.handleKnowledgeSearch).Methods("GET")

	// MCP 接口
	s.router.HandleFunc("/api/mcp", s.handleMCPRequest).Methods("POST")
}

// handleHealth 健康檢查
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now(),
	}
	s.writeJSON(w, response)
}

// handleDatabaseOverview 資料庫總覽
func (s *APIServer) handleDatabaseOverview(w http.ResponseWriter, r *http.Request) {
	// 獲取資料庫架構
	dbSchema, err := s.schemaReader.ReadSchema()
	if err != nil {
		s.writeError(w, "Failed to read database schema", http.StatusInternalServerError)
		return
	}

	// 進行統計分析
	analysis, err := s.analyzer.AnalyzeDatabase(dbSchema)
	if err != nil {
		s.writeError(w, "Failed to analyze database", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"database_info": dbSchema.DatabaseInfo,
		"table_count":   len(dbSchema.Tables),
		"analysis":      analysis.Summary,
		"tables":        analysis.TableAnalyses,
	}

	s.writeJSON(w, response)
}

// handleTableAnalysis 表格分析
func (s *APIServer) handleTableAnalysis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["tableName"]

	// 獲取資料庫架構
	dbSchema, err := s.schemaReader.ReadSchema()
	if err != nil {
		s.writeError(w, "Failed to read database schema", http.StatusInternalServerError)
		return
	}

	// 查找表格
	var table *schema.Table
	for _, t := range dbSchema.Tables {
		if t.Name == tableName {
			table = &t
			break
		}
	}

	if table == nil {
		s.writeError(w, "Table not found", http.StatusNotFound)
		return
	}

	// 分析表格
	recordCount, err := s.analyzer.GetRecordCount(tableName)
	if err != nil {
		s.writeError(w, "Failed to get record count", http.StatusInternalServerError)
		return
	}

	analysis := &analyzer.TableAnalysis{
		TableName:      tableName,
		RecordCount:    recordCount,
		ColumnAnalyses: make([]analyzer.ColumnAnalysis, 0, len(table.Columns)),
	}

	// 分析每個欄位
	for _, col := range table.Columns {
		colAnalysis, err := s.analyzer.AnalyzeColumn(tableName, col, recordCount)
		if err != nil {
			continue // 跳過有錯誤的欄位
		}
		analysis.ColumnAnalyses = append(analysis.ColumnAnalyses, *colAnalysis)
	}

	// 生成摘要
	analysis.Summary = s.analyzer.GenerateTableSummary(*table, analysis.ColumnAnalyses, recordCount)

	response := map[string]interface{}{
		"table":    table,
		"analysis": analysis,
	}

	s.writeJSON(w, response)
}

// handleTableAIAnalysis AI 表格分析
func (s *APIServer) handleTableAIAnalysis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["tableName"]

	// 檢查 AI 分析器是否可用
	if s.aiAnalyzer == nil {
		s.writeError(w, "AI analysis is not available. Please ensure LLM service is running.", http.StatusServiceUnavailable)
		return
	}

	// 進行 AI 分析
	analysis, err := s.aiAnalyzer.AnalyzeTablePurpose(r.Context(), tableName)
	if err != nil {
		s.writeError(w, fmt.Sprintf("Failed to analyze table: %v", err), http.StatusInternalServerError)
		return
	}

	// 儲存到知識庫
	if err := s.knowledgeBase.StoreAnalysis(analysis); err != nil {
		log.Printf("Warning: failed to store analysis in knowledge base: %v", err)
	}

	s.writeJSON(w, analysis)
}

// handleKnowledgeList 知識庫列表
func (s *APIServer) handleKnowledgeList(w http.ResponseWriter, r *http.Request) {
	tables := s.knowledgeBase.ListTables()
	stats := s.knowledgeBase.GetStats()

	response := map[string]interface{}{
		"tables": tables,
		"stats":  stats,
	}

	s.writeJSON(w, response)
}

// handleKnowledgeGet 獲取知識
func (s *APIServer) handleKnowledgeGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["tableName"]

	knowledge, err := s.knowledgeBase.GetKnowledge(tableName)
	if err != nil {
		s.writeError(w, "Knowledge not found", http.StatusNotFound)
		return
	}

	s.writeJSON(w, knowledge)
}

// handleKnowledgeSearch 搜尋知識
func (s *APIServer) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.writeError(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	results := s.knowledgeBase.SearchKnowledge(query)

	response := map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}

	s.writeJSON(w, response)
}

// handleMCPRequest MCP 請求處理
func (s *APIServer) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	var request mcp.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	response := s.mcpServer.HandleRequest(r.Context(), &request)
	s.writeJSON(w, response)
}

// Start 啟動服務器
func (s *APIServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// writeJSON 寫入 JSON 回應
func (s *APIServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError 寫入錯誤回應
func (s *APIServer) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"error": message,
		"status": statusCode,
	}
	json.NewEncoder(w).Encode(response)
}

// writeToFile 將數據寫入文件
func writeToFile(filename string, data []byte) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// runServer 啟動 HTTP 服務器
func runServer(db *sql.DB, cfg *config.Config) {
	// 建立 API 服務器
	server, err := NewAPIServer(db, cfg.Database.Type)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}

	// 啟動服務器
	log.Fatal(server.Start(cfg.App.Port))
}

// runPhase1 執行 Phase 1: 統計分析
func runPhase1(db *sql.DB, cfg *config.Config) {
	log.Println("=== Starting Phase 1: Statistical Analysis ===")

	// 建立架構讀取器
	schemaReader := schema.NewSchemaReader(db, cfg.Database.Type)

	// 建立資料分析器
	dataAnalyzer := analyzer.NewDatabaseAnalyzer(db, cfg.Database.Type)

	// 讀取資料庫架構
	dbSchema, err := schemaReader.ReadSchema()
	if err != nil {
		log.Fatalf("Failed to read database schema: %v", err)
	}

	log.Printf("Database: %s", dbSchema.DatabaseInfo.Name)
	log.Printf("Tables found: %d", len(dbSchema.Tables))

	// 進行統計分析
	analysis, err := dataAnalyzer.AnalyzeDatabase(dbSchema)
	if err != nil {
		log.Fatalf("Failed to analyze database: %v", err)
	}

	// 輸出結果
	fmt.Println("\n=== Phase 1 Analysis Results ===")
	fmt.Printf("Database: %s\n", dbSchema.DatabaseInfo.Name)
	fmt.Printf("Total Tables: %d\n", len(dbSchema.Tables))
	fmt.Printf("Analysis Summary: %+v\n", analysis.Summary)

	fmt.Println("\n=== Table Details ===")
	for _, tableAnalysis := range analysis.TableAnalyses {
		fmt.Printf("\nTable: %s\n", tableAnalysis.TableName)
		fmt.Printf("  Records: %d\n", tableAnalysis.RecordCount)
		fmt.Printf("  Columns: %d\n", len(tableAnalysis.ColumnAnalyses))
		fmt.Printf("  Summary: %+v\n", tableAnalysis.Summary)
	}

	// 保存到 JSON 文件
	output := map[string]interface{}{
		"database_info": dbSchema.DatabaseInfo,
		"table_count":   len(dbSchema.Tables),
		"analysis":      analysis.Summary,
		"tables":        analysis.TableAnalyses,
		"timestamp":     time.Now(),
	}

	outputFile := cfg.Schema.OutputFile
	if outputFile == "" {
		outputFile = "phase1_analysis.json"
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal analysis results: %v", err)
	}

	// 寫入文件
	err = writeToFile(outputFile, data)
	if err != nil {
		log.Fatalf("Failed to write analysis results to file: %v", err)
	}

	log.Printf("Phase 1 analysis completed. Results saved to %s", outputFile)
}

// runPhase2 執行 Phase 2: AI 分析
func runPhase2(db *sql.DB, cfg *config.Config) {
	log.Println("=== Starting Phase 2: AI Analysis ===")

	// 建立架構讀取器
	schemaReader := schema.NewSchemaReader(db, cfg.Database.Type)

	// 讀取資料庫架構
	dbSchema, err := schemaReader.ReadSchema()
	if err != nil {
		log.Fatalf("Failed to read database schema: %v", err)
	}

	// 建立 MCP 資料分析器
	mcpAnalyzer := mcp.NewDatabaseDataAnalyzer(db, cfg.Database.Type, dbSchema)

	// 建立 MCP Server
	mcpServer := mcp.NewMCPServer(dbSchema, mcpAnalyzer)

	// 建立 LLM 客戶端
	llmClient := ai.NewOpenAICompatibleClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model)

	// 測試 LLM 連接
	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = llmClient.GenerateAnalysis(testCtx, "test", nil)
	if err != nil {
		log.Fatalf("LLM service not available: %v", err)
	}

	// 建立 AI 分析器
	aiAnalyzer := ai.NewAIAnalyzer(mcpServer, llmClient)

	// 建立知識庫
	knowledgeBase, err := ai.NewKnowledgeBase("knowledge")
	if err != nil {
		log.Fatalf("Failed to create knowledge base: %v", err)
	}

	log.Printf("Starting AI analysis for %d tables...", len(dbSchema.Tables))

	// 分析每個表格
	results := make([]map[string]interface{}, 0, len(dbSchema.Tables))
	for _, table := range dbSchema.Tables {
		log.Printf("Analyzing table: %s", table.Name)

		analysis, err := aiAnalyzer.AnalyzeTablePurpose(context.Background(), table.Name)
		if err != nil {
			log.Printf("Warning: Failed to analyze table %s: %v", table.Name, err)
			continue
		}

		// 儲存到知識庫
		if err := knowledgeBase.StoreAnalysis(analysis); err != nil {
			log.Printf("Warning: failed to store analysis for table %s: %v", table.Name, err)
		}

		results = append(results, map[string]interface{}{
			"table_name": table.Name,
			"analysis":   analysis,
		})
	}

	// 保存結果
	output := map[string]interface{}{
		"database":     dbSchema.DatabaseInfo.Name,
		"total_tables": len(dbSchema.Tables),
		"analyzed_tables": len(results),
		"results":      results,
		"timestamp":    time.Now(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal AI analysis results: %v", err)
	}

	outputFile := "phase2_analysis.json"
	err = writeToFile(outputFile, data)
	if err != nil {
		log.Fatalf("Failed to write AI analysis results to file: %v", err)
	}

	log.Printf("Phase 2 AI analysis completed. Results saved to %s", outputFile)
	log.Printf("Analyzed %d out of %d tables", len(results), len(dbSchema.Tables))
}

func main() {
	// 命令行參數
	var command = flag.String("command", "server", "Command to run: server, phase1, phase2")
	var configPath = flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// 確保 context 包被使用
	_ = context.Background()

	// 載入 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with environment variables or defaults...")
	}

	// 載入配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 建立資料庫連接
	db, err := sql.Open(cfg.Database.Type, cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 測試資料庫連接
	if err := db.Ping(); err != nil {
		log.Printf("Warning: Failed to ping database: %v", err)
		log.Println("Continuing with limited functionality...")
	}

	log.Printf("Connected to %s database at %s:%d", cfg.Database.Type, cfg.Database.Host, cfg.Database.Port)

	// 根據命令執行不同的操作
	switch *command {
	case "server":
		runServer(db, cfg)
	case "phase1":
		runPhase1(db, cfg)
	case "phase2":
		runPhase2(db, cfg)
	default:
		log.Fatalf("Unknown command: %s. Available commands: server, phase1, phase2", *command)
	}
}
