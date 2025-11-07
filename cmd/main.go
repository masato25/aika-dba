package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/preparer"
	"github.com/masato25/aika-dba/pkg/query"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

func main() {
	var command = flag.String("command", "", "Command to run: prepare, query")
	var question = flag.String("question", "", "Question for query command")
	flag.Parse()

	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 連接到資料庫
	db, err := connectDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 創建知識管理器
	km, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create knowledge manager: %v", err)
	}
	defer km.Close()

	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)

	// 創建日誌器
	logger := log.New(os.Stdout, "[AikaDBA] ", log.LstdFlags)

	switch *command {
	case "prepare":
		runPrepare(db, km, logger)
	case "query":
		if *question == "" {
			log.Fatal("Question is required for query command")
		}
		runQuery(km, llmClient, db, *question, logger)
	default:
		fmt.Println("Usage:")
		fmt.Println("  -command prepare                    # Prepare knowledge base")
		fmt.Println("  -command query -question 'your question'  # Query knowledge base")
		os.Exit(1)
	}
}

func connectDatabase(cfg *config.Config) (*sql.DB, error) {
	var dsn string
	switch cfg.Database.Type {
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName)
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	db, err := sql.Open(cfg.Database.Type, dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func runPrepare(db *sql.DB, km *vectorstore.KnowledgeManager, logger *log.Logger) {
	logger.Println("Starting knowledge base preparation...")

	preparer := preparer.NewKnowledgePreparer(db, km, logger)
	if err := preparer.PrepareKnowledge(); err != nil {
		log.Fatalf("Knowledge preparation failed: %v", err)
	}

	logger.Println("Knowledge base preparation completed successfully")
}

func runQuery(km *vectorstore.KnowledgeManager, llmClient *llm.Client, db *sql.DB, question string, logger *log.Logger) {
	logger.Printf("Processing query: %s", question)

	queryInterface := query.NewQueryInterface(km, llmClient, db, logger)
	answer, err := queryInterface.Query(question)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Println("\nAnswer:")
	fmt.Println(answer)
}
