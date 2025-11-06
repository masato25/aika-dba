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
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/phases"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// APIServer API 服務器
type APIServer struct {
	router *mux.Router
}

// NewAPIServer 創建 API 服務器
func NewAPIServer(db *sql.DB, dbType string) (*APIServer, error) {
	server := &APIServer{
		router: mux.NewRouter(),
	}

	server.setupRoutes()
	return server, nil
}

// setupRoutes 設定路由
func (s *APIServer) setupRoutes() {
	// 健康檢查
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// 資料庫總覽 - 簡化版本
	s.router.HandleFunc("/api/database/overview", s.handleDatabaseOverview).Methods("GET")
}

// handleHealth 健康檢查
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now(),
	}
	s.writeJSON(w, response)
}

// handleDatabaseOverview 資料庫總覽 - 簡化版本
func (s *APIServer) handleDatabaseOverview(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Database analysis functionality is not yet implemented",
		"status":  "coming_soon",
		"time":    time.Now(),
	}
	s.writeJSON(w, response)
}

// Start 啟動服務器
func (s *APIServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, s.router)
}

// writeJSON 寫入 JSON 回應
func (s *APIServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
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

func main() {
	// 命令行參數
	var command = flag.String("command", "server", "Command to run: server, phase1, phase1_post, phase1_put, phase2, phase2_prefix, phase3, marketing, delete-vector")
	var configPath = flag.String("config", "config.yaml", "Path to config file")
	var phases = flag.String("phases", "phase3", "Comma-separated list of phases to delete (for delete-vector command)")
	var query = flag.String("query", "", "Natural language query for marketing command")
	flag.Parse()

	// 載入配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 建立資料庫連接
	log.Println("DEBUG: Opening database connection...")
	db, err := sql.Open(cfg.Database.Type, cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("DEBUG: Database connection opened")

	// 測試資料庫連接
	log.Println("DEBUG: Testing database connection with Ping()...")
	if err := db.Ping(); err != nil {
		log.Printf("Warning: Failed to ping database: %v", err)
		log.Println("Continuing with limited functionality...")
	} else {
		log.Println("DEBUG: Database ping successful")
	}

	log.Printf("Connected to %s database at %s:%d", cfg.Database.Type, cfg.Database.Host, cfg.Database.Port)

	// 根據命令執行不同的操作
	switch *command {
	case "server":
		runServer(db, cfg)
	case "phase1":
		runPhase1(db, cfg)
	case "phase1_post":
		runPhase1Post(cfg)
	case "phase1_put":
		runPhase1Put(cfg)
	case "phase2":
		runPhase2(db, cfg)
	case "phase2_prefix":
		runPhase2Prefix(cfg)
	case "phase3":
		runPhase3(cfg)
	case "marketing":
		runMarketingQuery(db, cfg, *query)
	case "delete-vector":
		runDeleteVectorData(cfg, *phases)
	default:
		log.Fatalf("Unknown command: %s. Available commands: server, phase1, phase1_post, phase1_put, phase2, phase2_prefix, phase3, marketing, delete-vector", *command)
	}
}
