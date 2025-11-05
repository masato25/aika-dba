# Aika DBA - Docker 測試環境

這個目錄包含完整的 Docker 測試環境，用於在真實的 PostgreSQL 資料庫上測試 Aika DBA 的 Phase 2 功能。

## 環境概述

- **PostgreSQL 15**: 完整的電子商務資料庫，包含 15+ 個互聯資料表
- **pgAdmin 4**: 網頁式資料庫管理介面
- **測試資料**: 10 個客戶、10 個產品、5 個訂單和相關業務資料

## 快速開始

### 1. 啟動 Docker 環境

```bash
# 從專案根目錄執行
docker-compose up -d
```

### 2. 檢查服務狀態

```bash
docker-compose ps
```

### 3. 運行完整測試

```bash
./test-docker-db.sh
```

## 服務訪問

- **PostgreSQL 資料庫**: `localhost:5432`
  - 用戶名: `ecommerce_user`
  - 密碼: `ecommerce_pass`
  - 資料庫: `ecommerce_db`

- **pgAdmin 介面**: http://localhost:8081
  - 郵箱: `admin@aika-dba.com`
  - 密碼: `admin123`

## 資料庫結構

### 主要資料表

- `customers` - 客戶資訊 (10 筆記錄)
- `categories` - 產品分類 (11 筆記錄，包含父子關係)
- `products` - 產品資訊 (10 筆記錄)
- `orders` - 訂單 (5 筆記錄)
- `order_items` - 訂單項目 (8 筆記錄)
- `payments` - 付款記錄 (4 筆記錄)
- `shipments` - 物流記錄 (3 筆記錄)
- `reviews` - 產品評價 (6 筆記錄)
- `inventory_transactions` - 庫存異動
- `coupons` - 優惠券

### 業務邏輯特點

- **複雜關係**: 客戶 → 訂單 → 付款/物流/評價
- **產品分類**: 父子分類結構 (電子產品 → 手機)
- **庫存管理**: 銷售導致的庫存異動
- **客戶忠誠度**: 基於消費金額的等級系統

## 測試 Phase 2 功能

### 1. 更新應用程式配置

確保 `config.yaml` 中的資料庫設定正確：

```yaml
database:
  type: "postgres"
  host: "localhost"
  port: 5432
  user: "ecommerce_user"
  password: "ecommerce_pass"
  dbname: "ecommerce_db"
```

### 2. 運行 AI 分析

```bash
# 編譯應用程式
go build -o bin/aika-dba ./cmd

# 運行 Phase 2 分析
./bin/aika-dba analyze --phase 2
```

### 3. 驗證分析結果

Phase 2 應該能夠：
- 識別外鍵關係 (orders.customer_id → customers.id)
- 發現約定俗成的關係 (products.category_id → categories.id)
- 生成業務邏輯總結
- 提供資料庫結構洞察

## 資料重置

如果需要重新初始化測試資料：

```bash
# 停止容器
docker-compose down

# 刪除資料卷（會清除所有資料）
docker volume rm aika-dba_postgres_data aika-dba_pgadmin_data

# 重新啟動
docker-compose up -d
```

## 故障排除

### 連接問題

1. 確保 Docker 容器正在運行：`docker-compose ps`
2. 檢查端口是否被占用：`lsof -i :5432`
3. 驗證資料庫健康狀態：`docker-compose exec postgres pg_isready -U ecommerce_user -d ecommerce_db`

### 資料問題

1. 檢查資料表結構：`docker-compose exec postgres psql -U ecommerce_user -d ecommerce_db -c "\dt"`
2. 驗證資料完整性：`docker-compose exec postgres psql -U ecommerce_user -d ecommerce_db -c "SELECT COUNT(*) FROM customers;"`

### 應用程式問題

1. 檢查配置檔案：`cat config.yaml`
2. 驗證環境變數：`env | grep DB_`
3. 查看應用程式日誌：`./bin/aika-dba --verbose`

## 開發建議

- 使用 pgAdmin 探索資料庫結構和關係
- 在進行 AI 分析前，先手動檢查幾個複雜的查詢
- 測試不同類型的業務邏輯推斷場景
- 驗證分析結果的準確性和完整性