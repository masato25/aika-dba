#!/bin/bash

# Aika DBA LLM 配置測試腳本
# 測試不同的 LLM 提供者配置

echo "=== Aika DBA LLM 配置測試 ==="

# 檢查 .env 文件
if [ ! -f ".env" ]; then
    echo "❌ .env 文件不存在，請從 .env.example 複製並配置"
    echo "   cp .env.example .env"
    exit 1
fi

echo "✅ .env 文件存在"

# 檢查環境變數
echo ""
echo "檢查環境變數："

if [ -n "$OPENAI_API_KEY" ]; then
    echo "✅ OPENAI_API_KEY 已設定"
else
    echo "❌ OPENAI_API_KEY 未設定"
fi

if [ -n "$OPENAI_BASE_URL" ]; then
    echo "✅ OPENAI_BASE_URL: $OPENAI_BASE_URL"
else
    echo "ℹ️  OPENAI_BASE_URL 未設定，將使用預設值"
fi

if [ -n "$LLM_MODEL" ]; then
    echo "✅ LLM_MODEL: $LLM_MODEL"
else
    echo "ℹ️  LLM_MODEL 未設定，將使用 config.yaml 中的值"
fi

# 檢查 config.yaml
echo ""
echo "檢查 config.yaml："

if [ ! -f "config.yaml" ]; then
    echo "❌ config.yaml 文件不存在"
    exit 1
fi

PROVIDER=$(grep "provider:" config.yaml | head -1 | cut -d'"' -f2)
MODEL=$(grep "model:" config.yaml | head -1 | cut -d'"' -f2)

echo "當前 LLM 配置："
echo "  Provider: $PROVIDER"
echo "  Model: $MODEL"

case $PROVIDER in
    "openai")
        echo "✅ 配置為使用 OpenAI API"
        if [ -z "$OPENAI_API_KEY" ]; then
            echo "⚠️  警告：OPENAI_API_KEY 環境變數未設定"
        fi
        ;;
    "local")
        echo "✅ 配置為使用本地 LLM 服務"
        ;;
    "ollama")
        echo "✅ 配置為使用 Ollama"
        ;;
    *)
        echo "❌ 不支援的 provider: $PROVIDER"
        ;;
esac

echo ""
echo "測試嵌入生成器："

# 檢查向量存儲配置
EMBEDDER_TYPE=$(grep "embedder_type:" config.yaml | head -1 | cut -d'"' -f2)
echo "嵌入類型: $EMBEDDER_TYPE"

if [ "$EMBEDDER_TYPE" = "openai" ]; then
    if [ -n "$OPENAI_API_KEY" ]; then
        echo "✅ OpenAI 嵌入已配置，可以測試"
        echo "運行測試: go run test_embedding_main.go"
    else
        echo "❌ OpenAI 嵌入需要 API 金鑰"
    fi
else
    echo "✅ 使用本地嵌入: $EMBEDDER_TYPE"
    echo "運行測試: go run test_embedding_main.go"
fi