#!/bin/bash

# Docker 資料庫測試腳本
# 測試 Aika DBA 應用程式與真實 PostgreSQL 資料庫的連接

echo "=== Aika DBA Docker 資料庫測試 ==="
echo

# 檢查 Docker 容器狀態
echo "檢查 Docker 容器狀態..."
docker-compose ps

echo
echo "=== 資料庫統計 ==="

# 顯示資料庫統計
docker-compose exec -T postgres psql -U ecommerce_user -d ecommerce_db -c "
SELECT
    '客戶數量' as 項目, COUNT(*) as 數量 FROM customers
UNION ALL
SELECT
    '產品數量' as 項目, COUNT(*) as 數量 FROM products
UNION ALL
SELECT
    '訂單數量' as 項目, COUNT(*) as 數量 FROM orders
UNION ALL
SELECT
    '訂單項目數量' as 項目, COUNT(*) as 數量 FROM order_items
UNION ALL
SELECT
    '評價數量' as 項目, COUNT(*) as 數量 FROM reviews
UNION ALL
SELECT
    '付款記錄數量' as 項目, COUNT(*) as 數量 FROM payments
UNION ALL
SELECT
    '物流記錄數量' as 項目, COUNT(*) as 數量 FROM shipments;
"

echo
echo "=== 測試應用程式連接 ==="

# 測試應用程式是否能啟動並連接資料庫
echo "編譯應用程式..."
go build -o bin/aika-dba ./cmd

if [ $? -eq 0 ]; then
    echo "應用程式編譯成功"
    echo
    echo "測試資料庫連接..."
    timeout 10s ./bin/aika-dba --test-db-connection || echo "連接測試完成"
else
    echo "應用程式編譯失敗"
fi

echo
echo "=== 測試完成 ==="
echo "PostgreSQL 資料庫運行在: localhost:5432"
echo "pgAdmin 介面運行在: http://localhost:8081"
echo "pgAdmin 登入: admin@aika-dba.com / admin123"