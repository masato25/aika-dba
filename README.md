# Aika DBA - 智慧資料分析系統

一個「資料分析 + AI 理解」的智慧分析系統，先透過統計分析了解資料庫概況，再使用 AI 深度理解每個表格的業務用途與定義。

## 🎯 核心目標

讓系統能夠：
1. **自動分析資料庫**：統計每個表格的資料量、欄位分佈、關聯性等
2. **AI 理解表格用途**：使用 LLM + MCP 分析資料樣本，生成每個表格的業務定義與用途說明

最終讓使用者能夠快速理解複雜資料庫的結構與業務邏輯。

## 🏗️ 系統架構

### 三層架構設計

| 層級 | 功能 | 技術組件 |
|------|------|----------|
| 1. 資料分析層 | 統計分析資料庫結構與資料分佈 | SQL Statistics + Data Profiling |
| 2. AI 理解層 | 使用 LLM + MCP 分析資料內容，生成表格用途定義 | LLM 模型 + MCP Server + Schema Analysis |
| 3. 知識整合層 | 整合分析結果與 AI 理解，建置知識庫 | Knowledge Base + API Interface |

### 資料流程

```
資料庫連接 → 統計分析 → 資料採樣 → AI 理解 (LLM + MCP) → 表格用途定義 → 知識庫整合
```

## 🚀 實作路線圖

### Phase 1: 資料庫統計分析 ⭐ (當前階段)
- [x] 建立 Schema Reader (已完成)
- [ ] 實作資料統計分析器
  - [ ] 表格記錄數統計
  - [ ] 欄位資料分佈分析
  - [ ] 欄位類型與空值統計
  - [ ] 表格間關聯分析
- [ ] 生成資料分析報告

### Phase 2: AI 理解與表格定義生成
- [ ] 設計 MCP Server 架構
- [ ] 實作資料採樣器 (從每個表格抽取代表性資料)
- [ ] 整合 LLM 模型分析資料內容
- [ ] 生成表格業務用途定義
- [ ] 建立表格知識庫

### Phase 3: 知識整合與查詢介面
- [ ] 整合分析結果與 AI 理解
- [ ] 建立 REST API 接口
- [ ] 實作查詢介面 (基於表格用途的搜尋)
- [ ] 支援多種資料庫廠商

## 🛠️ 技術棧

- **後端**: Golang (主架構)
- **AI/LLM**: OpenAI GPT-4/5, Qwen2.5-Coder (用於表格理解)
- **MCP**: Model Context Protocol (用於結構化資料分析)
- **資料庫**: 支援 MySQL, PostgreSQL, SQLite 等
- **分析**: 統計分析 + AI 理解
- **儲存**: JSON/YAML (分析結果與知識庫)

## 📁 專案結構

```
aika-dba/
├── cmd/                    # 應用程式入口
├── internal/
│   ├── schema/            # 資料庫結構解析
│   ├── analyzer/          # 資料統計分析器
│   ├── mcp/               # MCP Server 實作
│   ├── ai/                # AI 理解與表格定義生成
│   └── api/               # REST API
├── pkg/                   # 可重用套件
├── config/                # 設定檔案
├── scripts/               # 工具腳本
├── docs/                  # 文件
└── knowledge/             # 表格知識庫儲存
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

# 建置
go build -o bin/aika-dba ./cmd

# 執行
./bin/aika-dba
```

### 設定資料庫連接

編輯 `config/database.yaml`:

```yaml
database:
  type: mysql
  host: localhost
  port: 3306
  user: your_user
  password: your_password
  dbname: your_database
```

## 🤝 開發準則

請參考 [DEVELOPMENT_GUIDELINES.md](./DEVELOPMENT_GUIDELINES.md) 了解專案開發規範。

## 📝 使用範例

### 資料庫分析
```
輸入: 資料庫連接資訊
輸出: 
- customers 表：1,250 筆記錄，包含 id, name, email, created_at 等欄位
- orders 表：3,420 筆記錄，平均每位客戶 2.7 筆訂單
- products 表：156 筆記錄，價格範圍 $9.99-$999.99
```

### AI 表格理解
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

## 🔒 安全注意事項

- 資料採樣限制：只抽取必要樣本資料
- LLM 安全：控制輸入內容和輸出格式
- 資料庫權限：使用唯讀連接
- API 認證：實作適當的身份驗證機制

## 📈 未來擴展

- [ ] 支援更多資料庫類型
- [ ] 整合更多 LLM 模型
- [ ] 自動化 ETL 流程建議
- [ ] 資料品質評估
- [ ] 業務流程圖自動生成

## 📞 聯絡

如有問題或建議，請開啟 Issue 或聯絡開發團隊。