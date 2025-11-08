# Aika DBA - 智慧資料分析系統

一個「資料分析 + AI 理解」的智慧分析系統，先透過統計分析了解資料庫概況，再使用 AI 深度理解每個表格的業務用途與定義。

## 🎯 核心目標

讓系統能夠：
1. **自動分析資料庫**：統計每個表格的資料量、欄位分佈、關聯性等
2. **AI 理解表格用途**：使用 LLM + MCP 分析資料樣本，生成每個表格的業務定義與用途說明

最終讓使用者能夠快速理解複雜資料庫的結構與業務邏輯。

## 🏗️ 系統架構

### 四層架構設計

| 層級 | 功能 | 技術組件 |
|------|------|----------|
| 1. 資料分析層 | 統計分析資料庫結構與資料分佈 | SQL Statistics + Data Profiling |
| 2. AI 理解層 | 使用 LLM + MCP 分析資料內容，生成表格用途定義 | LLM 模型 + MCP Server + Schema Analysis |
| 3. 知識整合層 | 整合分析結果與 AI 理解，建置知識庫 | Knowledge Base + Vector Store |
| 4. 維度建模層 | 使用規則引擎進行資料倉儲維度建模 | Lua Rule Engine + Dimension Modeling |

### 資料流程

```
資料庫連接 → Phase 1統計分析 → Phase 2 AI理解 → Phase 3知識整合 → Phase 4維度建模
```

## 🚀 實作路線圖

### Phase 1: 資料庫統計分析 ⭐ (已完成)
- [x] 建立 Schema Reader (已完成)
- [x] 實作資料統計分析器
  - [x] 表格記錄數統計
  - [x] 欄位資料分佈分析
  - [x] 欄位類型與空值統計
  - [x] 表格間關聯分析 (外鍵、主鍵、約束)
  - [x] 索引分析
  - [x] 資料樣本採集
- [x] 生成資料分析報告 (JSON格式)
- [x] 向量知識庫整合

### Phase 2: AI 理解與表格定義生成 ⭐ (已完成)
- [x] 設計 MCP Server 架構
- [x] 實作資料採樣器 (從每個表格抽取代表性資料)
- [x] 整合 LLM 模型分析資料內容
- [x] 生成表格業務用途定義 (商業邏輯分析)
- [x] 建立表格知識庫
- [x] 支援多表格並行分析
- [x] 業務關係分析與分類

### Phase 3: 知識整合與查詢介面
- [ ] 整合分析結果與 AI 理解
- [ ] 建立 REST API 接口
- [ ] 實作查詢介面 (基於表格用途的搜尋)
- [ ] 支援多種資料庫廠商

### Phase 4: 維度建模與資料倉儲設計 ⭐ (已完成)
- [x] Lua 規則引擎實作
- [x] 智能維度分類 (人、時間、物體、事件、地點)
- [x] 事實表自動檢測
- [x] 業務導向的維度建模輸出
- [x] 星形/雪花模式支援
- [x] 維度建模報告生成

## 🛠️ 技術棧

- **後端**: Golang (主架構)
- **AI/LLM**: OpenAI GPT-4/5, Qwen2.5-Coder (用於表格理解)
- **MCP**: Model Context Protocol (用於結構化資料分析)
- **規則引擎**: Lua + Gopher-Lua (用於維度建模規則)
- **向量存儲**: SQLite + Vector Embeddings (用於知識檢索)
- **資料庫**: 支援 MySQL, PostgreSQL, SQLite 等
- **分析**: 統計分析 + AI 理解 + 維度建模
- **儲存**: JSON/YAML (分析結果與知識庫)

## 📁 專案結構

```
aika-dba/
├── cmd/                    # CLI 應用程式入口
│   └── main.go            # CLI 命令處理 (prepare, query)
├── webserver/             # Web 服務器入口
│   └── main.go            # Web 服務器啟動
├── internal/
│   └── app/               # 共享應用程式初始化
│       └── app.go         # App 結構和 NewApp() 函數
├── web/                   # Web 處理器和靜態檔案
│   ├── handlers.go        # HTTP 處理器
│   ├── index.html         # Web 介面
│   └── static/
│       └── style.css      # CSS 樣式
├── pkg/
│   ├── phases/            # 各階段處理邏輯
│   ├── vectorstore/       # 向量知識庫
│   ├── llm/               # LLM 客戶端
│   ├── query/             # 查詢介面
│   ├── preparer/          # 知識準備器
│   ├── progress/          # 進度追蹤
│   ├── analyzer/          # 資料分析器
│   └── mcp/               # MCP Server
├── config/                # 設定管理
├── data/                  # 向量資料庫儲存
├── bin/                   # 編譯後的二進位檔案
├── docs/                  # 文件
└── knowledge/             # 分析結果與知識庫
    ├── phase1_analysis.json    # Phase 1 統計分析結果
    ├── phase2_analysis.json    # Phase 2 AI 理解結果
    ├── phase4_dimensions.json  # Phase 4 維度建模結果
    ├── dimension_rules.lua     # 維度建模規則
    └── pre_phase3_summary.json # Phase 3 準備文件
```

## 🏃‍♂️ 快速開始

### 環境需求
- Go 1.25.3+
- Python 3.8+ (for 運算層)
- Docker (for 沙箱執行)

### 安裝與執行

```bash
# 複製專案
git clone https://github.com/masato25/aika-dba.git
cd aika-dba

# 安裝依賴
go mod download

# 建置 CLI 工具
go build -o bin/aika-dba ./cmd

# 建置 Web 服務器
go build -o bin/webserver ./webserver

# 執行 CLI 工具
./bin/aika-dba -command prepare                    # 準備知識庫
./bin/aika-dba -command query -question "你的問題"  # 查詢知識庫

# 執行 Web 服務器
./bin/webserver                                    # 啟動 Web 介面

# 開發模式 (熱重載)
make dev                                           # 使用 air 進行熱重載開發
```

### 開發模式說明

專案支援熱重載開發模式，可以在修改代碼後自動重新編譯和重啟服務器：

```bash
# 使用開發腳本 (推薦)
./dev.sh

# 或使用 Makefile
make dev

# 或直接使用 air
air
```

熱重載會監視以下檔案類型的變化：
- `.go` 檔案
- `.html` 模板檔案
- `.tmpl` 模板檔案

當這些檔案被修改時，air 會自動重新編譯並重啟服務器，大大提升開發效率。

**注意**: 開發腳本會自動檢查並安裝 air 工具，確保開發環境配置正確。

### 設定資料庫連接

1. 複製配置範本：
```bash
cp config.example.yaml config.yaml
```

2. 編輯 `config.yaml` 設定您的資料庫連接：

```yaml
database:
  type: postgres          # 或 mysql
  host: localhost         # 資料庫主機
  port: 5432             # 資料庫端口
  user: your_username    # 資料庫用戶名
  password: your_password # 資料庫密碼
  dbname: your_database   # 資料庫名稱
```

⚠️ **安全提醒**：
- `config.yaml` 包含敏感信息，已被加入 `.gitignore`
- 不要將真實的資料庫憑證提交到版本控制系統
- 建議使用環境變數或專用的密鑰管理服務

### LLM 配置設定

系統支援多種 LLM 提供者，可以輕鬆切換使用本地模型或雲端 API：

1. **複製環境變數範本**：
```bash
cp .env.example .env
```

2. **編輯 `.env` 文件設定您的 LLM 配置**：

#### 使用 OpenAI GPT (推薦)
```bash
# OpenAI API 設定
OPENAI_API_KEY=your_openai_api_key_here
OPENAI_BASE_URL=https://api.openai.com/v1
LLM_MODEL=gpt-4

# 更新 config.yaml
llm:
  provider: "openai"
  model: "gpt-4"
```

#### 使用本地 LLM 服務
```bash
# 本地 LLM 設定 (如使用 Ollama 或其他 OpenAI 兼容服務)
LLM_HOST=localhost
LLM_PORT=8080
LLM_MODEL=your-local-model

# 更新 config.yaml
llm:
  provider: "local"  # 或 "ollama"
  host: "localhost"
  port: 8080
```

#### 支援的 LLM 提供者
- `openai`: OpenAI 官方 API (GPT-3.5, GPT-4)
- `local`: 本地 OpenAI 兼容 API 服務
- `ollama`: Ollama 本地模型服務

## 🤝 開發準則

請參考 [DEVELOPMENT_GUIDELINES.md](./DEVELOPMENT_GUIDELINES.md) 了解專案開發規範。

## 📝 使用範例

### 資料庫分析 (Phase 1)
```
輸入: 資料庫連接資訊
輸出: 
- customers 表：1,250 筆記錄，包含 id, name, email, created_at 等欄位
- orders 表：3,420 筆記錄，平均每位客戶 2.7 筆訂單
- products 表：156 筆記錄，價格範圍 $9.99-$999.99
```

### AI 表格理解 (Phase 2)
```
輸入: customers 表格的資料樣本
輸出: 
"customers 表儲存了客戶基本資訊，包含：
- id: 唯一識別碼
- name: 客戶姓名
- email: 聯絡電子郵件
- created_at: 註冊時間

此表用於管理客戶關係，支援行銷活動和訂單追蹤"
```

### 維度建模分析 (Phase 4)
```
輸入: 業務表格分析結果
輸出:
人(People): dim_customer (顧客維度)
時間(Time): dim_date (時間維度)  
物體(Product): dim_product (產品維度)
事件(Event): 無
地點(Location): dim_location (地理位置維度)

事實表: fact_sales (銷售事實表) - 連接上述維度
```

## 🔒 安全注意事項

### 敏感資料保護
- **絕對不要提交敏感信息到版本控制**：
  - 資料庫憑證（用戶名、密碼、連接字串）
  - API 金鑰和權杖
  - 私有金鑰和證書
  - 個人識別信息（PII）
  - 內部伺服器 IP、主機名或端口
- **使用環境變數管理敏感配置**：
  - 資料庫連接詳情
  - API 金鑰和密鑰
  - 服務端點和端口
- **配置檔案安全**：
  - `config.yaml` 已加入 `.gitignore`
  - 使用 `config.example.yaml` 作為配置範本
  - 本地開發者請從範本複製並設置自己的配置

### 資料安全實踐
- 資料採樣限制：只抽取必要樣本資料
- LLM 安全：控制輸入內容和輸出格式，避免敏感資料外洩
- 資料庫權限：使用唯讀連接進行分析
- API 認證：實作適當的身份驗證機制
- 輸入驗證：驗證所有用戶輸入，防止注入攻擊
- 錯誤處理：不在錯誤訊息中暴露敏感信息

### 開發安全
- 定期進行安全稽核，檢查 git 歷史記錄
- 使用工具如 `git-secrets` 或 `truffleHog` 進行自動掃描
- 定期輪換憑證和密鑰
- 實作適當的存取控制和權限管理

## 📈 未來擴展

- [ ] **Phase 3 實現**: 知識整合與查詢介面
  - [ ] REST API 接口開發
  - [ ] 基於表格用途的智慧搜尋
  - [ ] 支援多種資料庫廠商
- [ ] **維度建模增強**: 
  - [ ] 緩慢變化的維度 (SCD) 支援
  - [ ] 彙總表自動生成
  - [ ] ETL 流程建議
- [ ] **AI 能力擴展**:
  - [ ] 支援更多 LLM 模型
  - [ ] 業務流程圖自動生成
  - [ ] 資料品質評估與建議
- [ ] **企業級功能**:
  - [ ] 多租戶支援
  - [ ] 即時分析儀表板
  - [ ] 資料血緣追蹤

## 📞 聯絡

如有問題或建議，請開啟 Issue 或聯絡開發團隊。