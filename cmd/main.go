package main

import (
"context"
"database/sql"
"flag"
"log"
"os"

_ "github.com/go-sql-driver/mysql"
_ "github.com/lib/pq"

"github.com/joho/godotenv"
"github.com/masato25/aika-dba/config"
"github.com/masato25/aika-dba/pkg/analyzer"
"github.com/masato25/aika-dba/pkg/llm"
"github.com/masato25/aika-dba/pkg/phases"
"github.com/masato25/aika-dba/pkg/vectorstore"
"github.com/masato25/aika-dba/pkg/web"
)

// runServer 啟動 HTTP 服務器
func runServer(db *sql.DB, cfg *config.Config) {
	web.RunServer(db, cfg)
}

// runPhase1 執行 Phase 1 分析
func runPhase1(db *sql.DB, cfg *config.Config) {
log.Println("DEBUG: Starting runPhase1 function")
dbAnalyzer := analyzer.NewDatabaseAnalyzer(db)
log.Println("DEBUG: DatabaseAnalyzer created")
runner, err := phases.NewPhase1Runner(dbAnalyzer, cfg)
if err != nil {
log.Fatalf("創建 Phase 1 執行器失敗: %v", err)
}
log.Println("DEBUG: Phase1Runner created")
if err := runner.Run(); err != nil {
log.Fatalf("Phase 1 分析失敗: %v", err)
}
runner.Close()
log.Println("Phase 1 分析完成！")
}

// runPhase2 執行 Phase 2 AI 分析
func runPhase2(db *sql.DB, cfg *config.Config) {
	runner, err := phases.NewPhase2Runner(cfg, db)
	if err != nil {
		log.Fatalf("創建 Phase 2 執行器失敗: %v", err)
	}

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 2 分析失敗: %v", err)
	}

	runner.Close()
	log.Println("Phase 2 分析完成！")
}

// runPhase3 執行 Phase 3 商業邏輯描述生成
func runPhase3(db *sql.DB, cfg *config.Config) {
	llmClient := llm.NewClient(cfg)

	vectorStore, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("創建向量存儲失敗: %v", err)
	}
	defer vectorStore.Close()

	runner := phases.NewPhase3Runner(cfg, llmClient, vectorStore)

	ctx := context.Background()
	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Phase 3 分析失敗: %v", err)
	}

	log.Println("Phase 3 分析完成！")
}

// runPhase4 執行 Phase 4 維度建模
func runPhase4(db *sql.DB, cfg *config.Config) {
	runner := phases.NewPhase4Runner(cfg, db)

	if err := runner.Run(); err != nil {
		log.Fatalf("Phase 4 分析失敗: %v", err)
	}

	log.Println("Phase 4 分析完成！")
}

// runPrepare 執行所有 phases
func runPrepare(db *sql.DB, cfg *config.Config) {
	log.Println("=== 開始執行完整準備流程 ===")

	// Phase 1: Schema 分析
	log.Println("執行 Phase 1...")
	runPhase1(db, cfg)

	// Phase 2: AI 商業邏輯分析
	log.Println("執行 Phase 2...")
	runPhase2(db, cfg)

	// Phase 3: 商業邏輯描述生成
	log.Println("執行 Phase 3...")
	runPhase3(db, cfg)

	// Phase 4: 維度建模
	log.Println("執行 Phase 4...")
	runPhase4(db, cfg)

	log.Println("=== 完整準備流程完成！ ===")
}

func main() {
	log.Println("Starting Aika DBA main function...")

	// 載入 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// 檢查是否有子命令
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <command> [options]\nAvailable commands: server, prepare, phase1, phase2, phase3, phase4", os.Args[0])
	}

	command := os.Args[1]
	args := os.Args[2:]

	// 命令行參數
	var configPath = flag.String("config", "config.yaml", "Path to config file")
	flag.CommandLine.Parse(args)

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
	switch command {
	case "server":
		runServer(db, cfg)
	case "prepare":
		runPrepare(db, cfg)
	case "phase1":
		runPhase1(db, cfg)
	case "phase2":
		runPhase2(db, cfg)
	case "phase3":
		runPhase3(db, cfg)
	case "phase4":
		runPhase4(db, cfg)
	default:
		log.Fatalf("Unknown command: %s. Available commands: server, prepare, phase1, phase2, phase3, phase4", command)
	}
}
