package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/masato25/aika-dba/config"
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

	fmt.Println("=== Testing Vector-Enhanced Phase 3 ===")

	// 創建並運行 Phase 3
	phase3Runner := phases.NewPhase3Runner(cfg)

	if err := phase3Runner.Run(); err != nil {
		log.Fatalf("Phase 3 failed: %v", err)
	}

	fmt.Println("=== Phase 3 Test completed successfully! ===")
}
