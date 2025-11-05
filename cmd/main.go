package main

import (
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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/phases"
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
	log.Printf("Starting API server on %s", addr)
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
	runner := phases.NewPhase1Runner(analyzer, cfg)

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 1 failed: %v", err)
	}
}

// runPhase2 執行 Phase 2: AI 分析
func runPhase2(db *sql.DB, cfg *config.Config) {
	runner := phases.NewPhase2Runner(cfg)

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 2 failed: %v", err)
	}
}

func main() {
	// 命令行參數
	var command = flag.String("command", "server", "Command to run: server, phase1, phase2")
	var configPath = flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

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
