# 重構架構設計：LLM 知識庫導向

## 概述

將專案重構為以 LLM 知識庫為中心的架構，支援自然語言查詢資料庫統計。捨棄複雜的 phases，專注於知識準備和查詢。

## 新架構組件

### 1. KnowledgePreparer (知識庫準備器)
負責掃描資料庫並準備知識庫：
- **數據提取**: 從資料庫提取表格結構、統計、樣本。
- **狀態檢測**: 自動檢測狀態欄位（如 status）的唯一值列表。
- **空表格過濾**: 忽略 row_count == 0 的表格。
- **知識存儲**: 將結構化數據存儲到 KnowledgeManager。

### 2. QueryInterface (查詢接口)
負責處理自然語言查詢：
- **查詢接收**: 接受用戶輸入的自然語言問題。
- **知識檢索**: 從 KnowledgeManager 檢索相關知識塊。
- **LLM 生成**: 將檢索結果作為上下文，調用 LLM 生成回答。

### 3. 主入口 (Main Entry)
簡單的 CLI 接口：
- `prepare`: 運行 KnowledgePreparer 準備知識庫。
- `query <question>`: 運行 QueryInterface 回答問題。

## 數據流程

1. **準備階段**: `prepare` → KnowledgePreparer → 存儲知識到向量庫。
2. **查詢階段**: `query "使用者的生日月份分布"` → QueryInterface → 檢索知識 → LLM 回答。

## 組件依賴

- **KnowledgeManager**: 核心向量存儲和檢索。
- **LLM Client**: 用於生成回答。
- **Database Client**: 用於數據提取。
- **Config**: 配置向量庫、LLM 等。

## 優勢

- **簡化**: 移除不必要的 phases，專注核心功能。
- **靈活性**: 支援任意自然語言查詢。
- **自動化**: 自動處理狀態列表和空表格。
- **擴展性**: 易於添加新知識來源或查詢類型。

此架構將專案轉向高效的 LLM 驅動資料庫查詢工具。</content>
<filePath>/Users/masato/Dev/app/go/aika-dba/docs/architecture_refactor.md