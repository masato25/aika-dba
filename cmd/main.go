package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	_ "github.com/lib/pq" // PostgreSQL 驅動程式
	"github.com/masato25/aika-dba/internal/schema"
	"gopkg.in/yaml.v3"
)

func main() {
	// 載入設定
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting %s v%s\n", config.App.Name, config.App.Version)
	fmt.Printf("Database: %s@%s:%d/%s\n", config.Database.User, config.Database.Host, config.Database.Port, config.Database.DBName)

	// 初始化資料庫連接
	db, err := schema.ConnectDatabase(config.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Successfully connected to database!")

	// 建立 SchemaReader
	reader := schema.NewSchemaReader(db, config.Database.Type)

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
	outputFile := config.Schema.OutputFile
	err = schemaData.SaveToFile(outputFile)
	if err != nil {
		log.Fatalf("Failed to save schema to file: %v", err)
	}

	fmt.Printf("Schema saved to: %s\n", outputFile)

	// 顯示前幾個表格
	fmt.Println("\nFirst few tables:")
	for i, table := range schemaData.Tables {
		if i >= 5 { // 只顯示前5個
			break
		}
		fmt.Printf("- %s (%s, %d columns)\n", table.Name, table.Type, len(table.Columns))
	}
}

type Config struct {
	Database schema.DatabaseConfig `yaml:"database"`
	App      AppConfig             `yaml:"app"`
	Schema   SchemaConfig          `yaml:"schema"`
	LLM      LLMConfig             `yaml:"llm"`
	Security SecurityConfig        `yaml:"security"`
	Logging  LoggingConfig         `yaml:"logging"`
}

type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

type SchemaConfig struct {
	OutputFile     string `yaml:"output_file"`
	MaxSamples     int    `yaml:"max_samples"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type LLMConfig struct {
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	BaseURL        string `yaml:"base_url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type SecurityConfig struct {
	EnableSQLSandbox bool     `yaml:"enable_sql_sandbox"`
	MaxQueryTime     int      `yaml:"max_query_time"`
	AllowedTables    []string `yaml:"allowed_tables"`
}

type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

func loadConfig() (*Config, error) {
	// 預設設定檔案路徑
	configFile := getEnv("CONFIG_FILE", "config.yaml")

	// 讀取設定檔案
	config := &Config{
		Database: schema.DatabaseConfig{
			Type:     "postgres",
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "password",
			DBName:   "postgres",
		},
		App: AppConfig{
			Name:    "Aika DBA",
			Version: "0.1.0",
			Port:    8080,
			Host:    "0.0.0.0",
		},
		Schema: SchemaConfig{
			OutputFile:     "schema_output.json",
			MaxSamples:     5,
			TimeoutSeconds: 30,
		},
		LLM: LLMConfig{
			Provider:       "openai",
			Model:          "gpt-4",
			APIKey:         "",
			BaseURL:        "",
			TimeoutSeconds: 60,
		},
		Security: SecurityConfig{
			EnableSQLSandbox: true,
			MaxQueryTime:     30,
			AllowedTables:    []string{},
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "json",
			Output:   "stdout",
			FilePath: "logs/aika-dba.log",
		},
	}

	// 如果設定檔案存在，讀取並覆蓋預設值
	if fileExists(configFile) {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
		}

		err = yaml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
		}
	}

	// 環境變數覆蓋
	applyEnvironmentOverrides(config)

	return config, nil
}

// applyEnvironmentOverrides 應用環境變數覆蓋
func applyEnvironmentOverrides(config *Config) {
	// 資料庫設定
	if v := os.Getenv("DB_TYPE"); v != "" {
		config.Database.Type = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		config.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			config.Database.Port = port
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		config.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		config.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		config.Database.DBName = v
	}

	// 應用程式設定
	if v := os.Getenv("APP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			config.App.Port = port
		}
	}
	if v := os.Getenv("APP_HOST"); v != "" {
		config.App.Host = v
	}

	// Schema 設定
	if v := os.Getenv("SCHEMA_OUTPUT_FILE"); v != "" {
		config.Schema.OutputFile = v
	}

	// LLM 設定
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		config.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		config.LLM.Model = v
	}
}

// getEnv 獲取環境變數，如果不存在則返回預設值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// fileExists 檢查檔案是否存在
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
