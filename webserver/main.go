package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/masato25/aika-dba/internal/app"
	"github.com/masato25/aika-dba/web"
)

func main() {
	// 載入 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// 創建應用程序實例
	app, err := app.NewApp()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	defer app.Close()

	// 設置 web 日誌器
	webLogger := log.New(app.Logger.Writer(), "[Web] ", log.LstdFlags)
	webLogger.Println("Starting web interface...")

	// 啟動 web 服務器
	web.RunServer(app.DB, app.Config)
}