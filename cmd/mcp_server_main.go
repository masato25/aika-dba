package main

import (
	"database/sql"
	"log"

	"github.com/joho/godotenv"
	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/mcp"
	_ "github.com/lib/pq"
)

func main() {
	// 載入 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
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
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Printf("Connected to %s database", cfg.Database.Type)

	// 創建並啟動 MCP 服務器
	mcpServer := mcp.NewMCPServer(db)

	log.Println("Starting MCP Server... Press Ctrl+C to stop")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("MCP Server failed: %v", err)
	}
}