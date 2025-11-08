#!/bin/bash

# Aika DBA 開發腳本
# 用於啟動熱重載開發模式

set -e

echo "🚀 啟動 Aika DBA 開發模式 (熱重載)..."

# 檢查 air 是否安裝
if ! command -v air &> /dev/null && ! [ -f "/Users/masato/go/bin/air" ]; then
    echo "❌ air 未安裝，正在安裝..."
    go install github.com/air-verse/air@latest
fi

# 確保 tmp 目錄存在
mkdir -p tmp

# 啟動 air
if command -v air &> /dev/null; then
    air
else
    /Users/masato/go/bin/air
fi