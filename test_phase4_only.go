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

	fmt.Println("=== Testing Vector-Enhanced Phase 4 ===")

	// 創建並運行 Phase 4
	phase4Runner := phases.NewPhase4Runner(cfg, db)

	if err := phase4Runner.Run(); err != nil {
		log.Fatalf("Phase 4 failed: %v", err)
	}

	fmt.Println("=== Phase 4 Test completed successfully! ===")
}
