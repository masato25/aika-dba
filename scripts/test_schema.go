package main

import (
	"fmt"
	"log"
	"os"

	"github.com/masato25/aika-dba/internal/schema"
)

func main() {
	// 從環境變數獲取資料庫連接資訊
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "password")
	dbName := getEnv("DB_NAME", "test")

	fmt.Printf("Connecting to PostgreSQL: %s:%s/%s\n", dbHost, dbPort, dbName)

	// 連接到資料庫
	db, err := schema.ConnectDatabase(schema.DatabaseConfig{
		Type:     "postgres",
		Host:     dbHost,
		Port:     5432,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Successfully connected to database!")

	// 建立 SchemaReader
	reader := schema.NewSchemaReader(db, "postgres")

	// 讀取 schema
	fmt.Println("Reading database schema...")
	schemaData, err := reader.ReadSchema()
	if err != nil {
		log.Fatalf("Failed to read schema: %v", err)
	}

	fmt.Printf("Schema collected successfully!\n")
	fmt.Printf("- Database: %s (%s)\n", schemaData.DatabaseInfo.Name, schemaData.DatabaseInfo.Version)
	fmt.Printf("- Tables: %d\n", schemaData.Metadata.TotalTables)
	fmt.Printf("- Columns: %d\n", schemaData.Metadata.TotalColumns)
	fmt.Printf("- Relationships: %d\n", len(schemaData.Relationships))
	fmt.Printf("- Collection time: %d ms\n", schemaData.Metadata.CollectionDurationMs)

	// 儲存到檔案
	outputFile := "schema_output.json"
	err = schemaData.SaveToFile(outputFile)
	if err != nil {
		log.Fatalf("Failed to save schema to file: %v", err)
	}

	fmt.Printf("Schema saved to: %s\n", outputFile)

	// 顯示前幾個表格
	fmt.Println("\nFirst few tables:")
	for i, table := range schemaData.Tables {
		if i >= 3 { // 只顯示前3個
			break
		}
		fmt.Printf("- %s (%s, %d columns)\n", table.Name, table.Type, len(table.Columns))
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}