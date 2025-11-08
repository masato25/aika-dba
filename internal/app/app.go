package app

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// App 應用程序實例
type App struct {
	Config           *config.Config
	DB               *sql.DB
	LLMClient        *llm.Client
	KnowledgeManager *vectorstore.KnowledgeManager
	Logger           *log.Logger
}

// NewApp 創建新的應用程序實例
func NewApp() (*App, error) {
	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 連接到資料庫
	db, err := connectDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 創建知識管理器
	km, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create knowledge manager: %w", err)
	}

	// 創建 LLM 客戶端
	llmClient := llm.NewClient(cfg)

	// 創建日誌器
	logger := log.New(os.Stdout, "[AikaDBA] ", log.LstdFlags)

	return &App{
		Config:           cfg,
		DB:               db,
		LLMClient:        llmClient,
		KnowledgeManager: km,
		Logger:           logger,
	}, nil
}

// Close 關閉應用程序資源
func (a *App) Close() error {
	if a.KnowledgeManager != nil {
		a.KnowledgeManager.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
	return nil
}

// connectDatabase 連接到資料庫
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
