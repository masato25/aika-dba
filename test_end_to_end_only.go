package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/phases"
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

	fmt.Println("=== End-to-End Vector-Enhanced Phase Testing ===")

	// Phase 1: 統計分析
	fmt.Println("\n--- Phase 1: Statistical Analysis ---")
	dbAnalyzer := analyzer.NewDatabaseAnalyzer(db)
	phase1Runner, err := phases.NewPhase1Runner(dbAnalyzer, cfg)
	if err != nil {
		log.Fatalf("Failed to create phase1 runner: %v", err)
	}
	if err := phase1Runner.Run(); err != nil {
		log.Fatalf("Phase 1 failed: %v", err)
	}
	if err := phase1Runner.Close(); err != nil {
		log.Printf("Warning: Failed to close phase1 runner: %v", err)
	}

	// Phase 2: AI 業務邏輯分析
	fmt.Println("\n--- Phase 2: AI Business Logic Analysis ---")
	phase2Runner, err := phases.NewPhase2Runner(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create phase2 runner: %v", err)
	}
	if err := phase2Runner.Run(); err != nil {
		log.Fatalf("Phase 2 failed: %v", err)
	}
	if err := phase2Runner.Close(); err != nil {
		log.Printf("Warning: Failed to close phase2 runner: %v", err)
	}

	// Phase 3: 自動生成維度規則
	fmt.Println("\n--- Phase 3: Auto-Generate Dimension Rules ---")
	phase3Runner := phases.NewPhase3Runner(cfg)
	if err := phase3Runner.Run(); err != nil {
		log.Fatalf("Phase 3 failed: %v", err)
	}

	// Phase 4: Lua 規則引擎維度建模
	fmt.Println("\n--- Phase 4: Lua Rule Engine Dimension Modeling ---")
	phase4Runner := phases.NewPhase4Runner(cfg, db)
	if err := phase4Runner.Run(); err != nil {
		log.Fatalf("Phase 4 failed: %v", err)
	}

	fmt.Println("\n=== All Phases Completed Successfully! ===")
	fmt.Println("Vector-enhanced knowledge flow:")
	fmt.Println("Phase 1 → Vector Store → Phase 2 → Vector Store → Phase 3 → Vector Store → Phase 4")
	fmt.Println("Check knowledge/ directory for generated files and vector database for stored knowledge")
}
