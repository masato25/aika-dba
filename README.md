# Aika DBA - 智慧資料分析系統

一個「資料理解 + 自然語言轉 SQL/程式的智慧分析系統」，讓使用者只需輸入自然語言就能自動產生 SQL、執行查詢並顯示結果。

## 🎯 核心目標

讓使用者不用懂資料庫，只要輸入自然語言（例如：'幫我查出2024年銷售額最高的前10個客戶'），系統就能自動產生 SQL、執行、並顯示結果。

同時支援：
- 根據不同廠商資料庫的結構自動適應
- 計算指標、比率、增長率等簡單運算
- 結果視覺化（表格/圖表）

## 🏗️ 系統架構

### 三層架構設計

| 層級 | 功能 | 技術組件 |
|------|------|----------|
| 1. 資料理解層 | 解析資料庫結構、建立 schema 認識 | SQL Introspection + Metadata Extractor |
| 2. 語意轉換層 | 將自然語言轉換為 SQL 查詢或程式 | LLM 模型 + Schema Prompt |
| 3. 執行與視覺化層 | 執行 SQL 並顯示結果 | Query Executor + Chart Generator |

### 資料流程

```
自然語言輸入 → 語意轉換 (LLM) → SQL/程式生成 → 安全執行 → 結果視覺化
```

## 🚀 實作路線圖

### Phase 1: 資料庫結構學習
- [ ] 建立 Schema Reader
- [ ] 自動解析資料庫結構（tables, columns, relationships）
- [ ] 將 schema 存為 JSON 格式供 LLM 使用

### Phase 2: 自然語言轉 SQL
- [ ] 設計 LLM Prompt 模板
- [ ] 整合 OpenAI GPT 或本地模型 (Qwen2.5-Coder)
- [ ] 實現 NL → SQL 轉換
- [ ] 添加輸出格式驗證

### Phase 3: SQL 執行與結果呈現
- [ ] 實現安全的 SQL 執行（prepared statement）
- [ ] 建立 REST API 接口
- [ ] 基本結果表格顯示
- [ ] 支援多種資料庫廠商

### Phase 4: 延伸運算層
- [ ] 支援簡單 Python/Golang 程式生成
- [ ] 沙箱環境執行（Docker/subprocess）
- [ ] 圖表生成整合
- [ ] 複雜運算支援（增長率、指標計算）

## 🛠️ 技術棧

- **後端**: Golang (主架構)
- **AI/LLM**: OpenAI GPT-4/5, Qwen2.5-Coder
- **資料庫**: 支援 MySQL, PostgreSQL, SQLite 等
- **前端**: React (視覺化界面)
- **運算**: Python (pandas, numpy for 複雜計算)
- **容器化**: Docker (安全沙箱)

## 📁 專案結構

```
aika-dba/
├── cmd/                    # 應用程式入口
├── internal/
│   ├── schema/            # 資料庫結構解析
│   ├── nlp/               # 自然語言處理
│   ├── executor/          # SQL 執行器
│   └── api/               # REST API
├── pkg/                   # 可重用套件
├── web/                   # 前端資源
├── config/                # 設定檔案
├── scripts/               # 工具腳本
└── docs/                  # 文件
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

### 基本查詢
```
輸入: "幫我查出2024年銷售額最高的前10個客戶"
輸出: 自動生成 SQL 並執行，顯示結果表格
```

### 運算查詢
```
輸入: "計算每個月平均銷售成長率"
輸出: 生成 Python 程式，執行運算並顯示圖表
```

## 🔒 安全注意事項

- SQL 注入防護：使用 prepared statement
- 程式執行沙箱：限制權限和資源使用
- API 認證：實作適當的身份驗證機制

## 📈 未來擴展

- [ ] 支援更多資料庫類型
- [ ] 整合更多 LLM 模型
- [ ] 進階視覺化圖表
- [ ] 機器學習預測功能
- [ ] 多語言支援

## 📞 聯絡

如有問題或建議，請開啟 Issue 或聯絡開發團隊。