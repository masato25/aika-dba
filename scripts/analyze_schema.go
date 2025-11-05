package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/masato25/aika-dba/internal/schema"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run scripts/test_analyzer.go <config_file>")
		fmt.Println("範例: go run scripts/test_analyzer.go config.yaml")
		os.Exit(1)
	}

	// 讀取 schema（實際環境中這裡會連接到真實資料庫）
	// 這裡我們直接載入之前儲存的 schema 文件來測試
	schemaData, err := os.ReadFile("my_schema.json")
	if err != nil {
		log.Fatalf("讀取 schema 文件失敗: %v", err)
	}

	var dbSchema schema.DatabaseSchema
	if err := json.Unmarshal(schemaData, &dbSchema); err != nil {
		log.Fatalf("解析 schema 失敗: %v", err)
	}

	fmt.Printf("開始分析資料庫: %s\n", dbSchema.DatabaseInfo.Name)
	fmt.Printf("表格數量: %d\n", len(dbSchema.Tables))

	// 模擬分析結果
	fmt.Println("\n=== 模擬分析結果 ===")

	for _, table := range dbSchema.Tables {
		fmt.Printf("\n表格: %s\n", table.Name)
		fmt.Printf("  欄位數量: %d\n", len(table.Columns))
		fmt.Printf("  預估記錄數: %d\n", table.EstimatedRows)

		// 顯示主要欄位
		fmt.Println("  主要欄位:")
		for _, col := range table.Columns {
			pkFlag := ""
			if col.IsPrimaryKey {
				pkFlag = " (主鍵)"
			}
			fkFlag := ""
			if col.IsForeignKey {
				fkFlag = " (外鍵)"
			}
			fmt.Printf("    - %s: %s%s%s\n", col.Name, col.Type, pkFlag, fkFlag)
		}
	}

	fmt.Println("\n=== 分析完成 ===")
	fmt.Println("注意: 這是模擬分析。要獲得真實統計數據，需要連接實際資料庫。")
}