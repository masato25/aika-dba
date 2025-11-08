package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/phases"
	_ "github.com/lib/pq"
)

func main() {
	// 載入環境變數
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Failed to load .env file: %v", err)
	}

	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 連接到數據庫
	db, err := sql.Open("postgres", cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 測試連接
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Database connected successfully")

	// 創建營銷查詢執行器
	runner := phases.NewMarketingQueryRunner(cfg, db)

	// 測試查詢：會員生日月份分佈
	testQuery := "會員生日月份分佈"
	fmt.Printf("Testing query: %s\n", testQuery)

	result, err := runner.ExecuteMarketingQuery(testQuery)
	if err != nil {
		log.Printf("Failed to execute query: %v", err)
		return
	}

	fmt.Printf("Generated SQL: %s\n", result.SQLQuery)
	fmt.Printf("Explanation: %s\n", result.Explanation)

	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	} else {
		fmt.Printf("Results count: %d\n", len(result.Results))
		for i, row := range result.Results {
			if i >= 5 { // 只顯示前5個結果
				fmt.Printf("... and %d more results\n", len(result.Results)-5)
				break
			}
			fmt.Printf("Row %d: %v\n", i+1, row)
		}
	}
}