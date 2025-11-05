package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config 應用程式配置結構
type Config struct {
	Database    DatabaseConfig    `yaml:"database"`
	App         AppConfig         `yaml:"app"`
	Schema      SchemaConfig      `yaml:"schema"`
	LLM         LLMConfig         `yaml:"llm"`
	VectorStore VectorStoreConfig `yaml:"vectorstore"`
	Security    SecurityConfig    `yaml:"security"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// DatabaseConfig 資料庫配置
type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// AppConfig 應用程式配置
type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

// SchemaConfig Schema 收集配置
type SchemaConfig struct {
	OutputFile     string `yaml:"output_file"`
	MaxSamples     int    `yaml:"max_samples"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	BaseURL        string `yaml:"base_url"`
	Host           string `yaml:"host"` // 本地 LLM 主機
	Port           int    `yaml:"port"` // 本地 LLM 端口
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// VectorStoreConfig 向量存儲配置
type VectorStoreConfig struct {
	Enabled            bool   `yaml:"enabled"`
	DatabasePath       string `yaml:"database_path"`
	EmbedderType       string `yaml:"embedder_type"`
	QwenModelPath      string `yaml:"qwen_model_path"`
	EmbeddingDimension int    `yaml:"embedding_dimension"`
	ChunkSize          int    `yaml:"chunk_size"`
	ChunkOverlap       int    `yaml:"chunk_overlap"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableSQLSandbox bool     `yaml:"enable_sql_sandbox"`
	MaxQueryTime     int      `yaml:"max_query_time"`
	AllowedTables    []string `yaml:"allowed_tables"`
}

// LoggingConfig 記錄配置
type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

// LoadConfig 載入配置檔案
func LoadConfig(configPath string) (*Config, error) {
	// 如果沒有指定配置文件，使用預設路徑
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 讀取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// 解析 YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 環境變數覆蓋
	config = overrideWithEnv(config)

	return &config, nil
}

// overrideWithEnv 使用環境變數覆蓋配置
func overrideWithEnv(config Config) Config {
	// 資料庫配置
	if dbType := os.Getenv("DB_TYPE"); dbType != "" {
		config.Database.Type = dbType
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			config.Database.Port = port
		}
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.User = dbUser
	}
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		config.Database.Password = dbPassword
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.DBName = dbName
	}

	// 應用程式配置
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.App.Port = p
		}
	}

	// LLM 配置
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config.LLM.APIKey = apiKey
	}
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.LLM.BaseURL = baseURL
	}
	if model := os.Getenv("LLM_MODEL"); model != "" {
		config.LLM.Model = model
	}
	if llmHost := os.Getenv("LLM_HOST"); llmHost != "" {
		config.LLM.Host = llmHost
	}
	if llmPort := os.Getenv("LLM_PORT"); llmPort != "" {
		if port, err := strconv.Atoi(llmPort); err == nil {
			config.LLM.Port = port
		}
	}

	return config
}

// GetDatabaseDSN 獲取資料庫連接字串
func (c *Config) GetDatabaseDSN() string {
	switch c.Database.Type {
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password, c.Database.DBName)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.DBName)
	default:
		return ""
	}
}
