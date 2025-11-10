# Aika DBA 詳細使用指南

## 目錄
- [系統概述](#系統概述)
- [部署指南](#部署指南)
- [配置指南](#配置指南)
- [功能說明](#功能說明)
- [開發指南](#開發指南)
- [故障排除](#故障排除)

## 系統概述

Aika DBA 是一個結合 AI 的資料庫分析工具，主要用於：

1. 自動分析資料庫結構和資料分布
2. 利用 AI 理解表格的業務用途
3. 生成資料庫的知識庫
4. 提供智能查詢介面

### 架構設計

系統分為四個主要階段：

1. **Phase 1: 資料庫統計分析**
   - 表格記錄數統計
   - 欄位資料分布分析
   - 表格關聯分析

2. **Phase 2: AI 理解與定義生成**
   - 利用 LLM 分析資料內容
   - 生成表格業務用途定義
   - 建立表格知識庫

3. **Phase 3: 知識整合**
   - 整合統計分析和 AI 理解結果
   - 提供 REST API 查詢介面

4. **Phase 4: 維度建模**
   - 自動化維度分類
   - 生成維度建模建議
   - 輸出建模報告

## 部署指南

### 系統需求

- Go 1.25.3+
- PostgreSQL 或 MySQL
- 16GB+ RAM（推薦）
- 磁盤空間：取決於數據庫大小

### 安裝步驟

1. **下載和編譯**：
```bash
git clone https://github.com/masato25/aika-dba.git
cd aika-dba
make build
```

2. **初始化配置**：
```bash
cp config.example.yaml config.yaml
```

3. **設置環境變數**：
```bash
cp .env.example .env
# 編輯 .env 文件設置必要的環境變數
```

4. **啟動服務**：
```bash
./bin/aika-dba -command prepare  # 準備知識庫
./bin/webserver                  # 啟動 Web 服務
```

## 配置指南

### 資料庫配置

在 `config.yaml` 中配置資料庫連接：

```yaml
database:
  type: "postgres"          # postgres 或 mysql
  host: "localhost"         
  port: 5432               
  user: "your_username"    
  password: "your_password" 
  dbname: "your_database"   
```

### LLM 配置

系統支援多種 LLM 提供者：

1. **OpenAI**：
```yaml
llm:
  provider: "openai"
  model: "gpt-4"
  api_key: ""  # 使用環境變數 OPENAI_API_KEY
  temperature: 0.7
  top_p: 0.9
```

2. **本地 LLM（Local OpenAI Compatible）**：
```yaml
llm:
  provider: "local"
  model: "mistral-7b"
  host: "localhost"
  port: 8080
```

3. **Ollama**：
```yaml
llm:
  provider: "ollama"
  model: "llama2"
  host: "localhost"
  port: 11434
```

4. **llama.cpp**：
```yaml
llm:
  provider: "llamacpp"
  model: "mistral-7b"
  host: "localhost"
  port: 8080
  temperature: 0.7
  top_p: 0.9
  top_k: 40
```

### 向量存儲配置

```yaml
vectorstore:
  enabled: true
  database_path: "data/vectorstore"
  embedder_type: "openai"  # openai, qwen, simple
  embedding_dimension: 1536
```

## 功能說明

### 1. 資料庫分析（Phase 1）

使用命令：
```bash
./bin/aika-dba -command analyze
```

輸出：
- `knowledge/phase1_analysis.json`：詳細的資料庫分析報告
- 統計信息：表格大小、欄位分布、關聯等

### 2. AI 理解（Phase 2）

使用命令：
```bash
./bin/aika-dba -command understand
```

功能：
- 分析表格數據樣本
- 生成業務邏輯描述
- 建立知識圖譜

### 3. 知識查詢（Phase 3）

REST API 端點：
- `GET /api/v1/tables`：列出所有表格
- `GET /api/v1/tables/{name}`：獲取特定表格信息
- `POST /api/v1/query`：執行智能查詢

### 4. 維度建模（Phase 4）

使用命令：
```bash
./bin/aika-dba -command model
```

輸出：
- 維度表建議
- 事實表識別
- 關聯模式建議

## 開發指南

### 開發環境設置

1. **安裝依賴**：
```bash
make deps
```

2. **運行測試**：
```bash
make test
```

3. **代碼檢查**：
```bash
make lint
```

### 擴展開發

1. **添加新的 LLM 提供者**：
   - 在 `pkg/llm` 中實現新的客戶端
   - 更新 `Client` 介面
   - 添加配置選項

2. **自定義分析規則**：
   - 修改 `pkg/analyzer` 中的分析邏輯
   - 添加新的分析器介面

3. **添加新的 API 端點**：
   - 在 `web/handlers.go` 中添加處理器
   - 更新路由配置

## 故障排除

### 常見問題

1. **連接錯誤**：
   - 檢查資料庫配置
   - 確認網絡連接
   - 驗證認證信息

2. **LLM 錯誤**：
   - 檢查 API 密鑰
   - 確認模型可用性
   - 查看錯誤日誌

3. **效能問題**：
   - 調整批次大小
   - 優化查詢參數
   - 增加資源配置

### 日誌收集

日誌位置：
- 應用日誌：`logs/aika-dba.log`
- 錯誤日誌：`logs/error.log`
- Web 訪問日誌：`logs/access.log`

### 監控指標

主要指標：
- API 響應時間
- LLM 請求延遲
- 資料庫查詢時間
- 記憶體使用情況

## 支援和反饋

- GitHub Issues：用於 bug 報告和功能請求
- 技術支援郵箱：support@example.com
- 開發者文檔：[docs/](./docs/)