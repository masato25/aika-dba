# 設定說明

## 設定檔案

Aika DBA 使用 `config.yaml` 作為預設設定檔案。你可以自定義以下設定：

### 基本使用

```bash
# 使用預設設定檔案
./bin/aika-dba

# 指定自定義設定檔案
CONFIG_FILE=my-config.yaml ./bin/aika-dba
```

### 環境變數覆蓋

支援以下環境變數來覆蓋設定：

#### 資料庫設定
- `DB_TYPE`: 資料庫類型 (postgres/mysql)
- `DB_HOST`: 資料庫主機
- `DB_PORT`: 資料庫端口
- `DB_USER`: 資料庫用戶名
- `DB_PASSWORD`: 資料庫密碼
- `DB_NAME`: 資料庫名稱

#### 應用程式設定
- `APP_PORT`: API 服務器端口
- `APP_HOST`: API 服務器主機

#### Schema 設定
- `SCHEMA_OUTPUT_FILE`: Schema 輸出檔案名稱

#### LLM 設定
- `LLM_API_KEY`: LLM API 金鑰
- `LLM_MODEL`: LLM 模型名稱

### 使用範例

```bash
# 使用環境變數覆蓋資料庫設定
DB_HOST=192.168.1.100 DB_PORT=3306 DB_TYPE=mysql ./bin/aika-dba

# 自定義輸出檔案
SCHEMA_OUTPUT_FILE=my_custom_schema.json ./bin/aika-dba

# 同時設定多個參數
DB_HOST=prod-db.company.com DB_USER=app_user LLM_API_KEY=sk-... ./bin/aika-dba
```

### 設定檔案範例

```yaml
database:
  type: "postgres"
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "postgres"

app:
  name: "Aika DBA"
  version: "0.1.0"
  port: 8080
  host: "0.0.0.0"

schema:
  output_file: "schema_output.json"
  max_samples: 5
  timeout_seconds: 30
```