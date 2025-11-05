package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/phases"
	_ "github.com/lib/pq"
)

func main() {
	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 連接資料庫
	db, err := sql.Open("postgres", cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 創建資料庫分析器
	dbAnalyzer := analyzer.NewDatabaseAnalyzer(db)

	fmt.Println("=== Testing Vector-Enhanced Phase 1 ===")

	// 創建並運行 Phase 1
	phase1Runner, err := phases.NewPhase1Runner(dbAnalyzer, cfg)
	if err != nil {
		log.Fatalf("Failed to create phase1 runner: %v", err)
	}

	if err := phase1Runner.Run(); err != nil {
		log.Fatalf("Phase 1 failed: %v", err)
	}

	// 關閉 Phase 1
	if err := phase1Runner.Close(); err != nil {
		log.Printf("Warning: Failed to close phase1 runner: %v", err)
	}

	fmt.Println("=== Testing Vector-Enhanced Phase 2 ===")

	// 創建並運行 Phase 2
	phase2Runner, err := phases.NewPhase2Runner(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create phase2 runner: %v", err)
	}

	if err := phase2Runner.Run(); err != nil {
		log.Fatalf("Phase 2 failed: %v", err)
	}

	// 關閉 Phase 2
	if err := phase2Runner.Close(); err != nil {
		log.Printf("Warning: Failed to close phase2 runner: %v", err)
	}

	fmt.Println("=== Test completed successfully! ===")
}