# LLM 知識庫指南：支援自然語言查詢

## 概述

本指南說明如何使用 `KnowledgeManager` 組件建立良好的知識庫，讓 LLM 能夠支援自然語言查詢（如“使用者的生日月份分布”）。知識庫基於向量存儲，允許語義搜尋，而非精確匹配。

## KnowledgeManager 組件概述

`KnowledgeManager` 是統一管理向量知識的組件，用於存儲和檢索不同階段（phases）的資料庫分析知識。它整合了向量存儲、嵌入生成器和知識分塊器。

### 主要功能
- **初始化**: 根據配置創建嵌入生成器（支援 Qwen 或簡單哈希嵌入）。
- **存儲知識**: 將結構化知識轉換為文本，分塊並生成向量嵌入。
- **檢索知識**: 基於相似度檢索相關知識塊。
- **統計和維護**: 提供知識統計、刪除和導出功能。

## 準備知識庫的步驟

### 1. 數據提取和準備
- 從資料庫提取關鍵資訊：表格結構、統計數據、樣本數據。
- 避免存儲原始數據，只存儲元數據和統計以保護隱私。

### 2. 知識結構化
將數據組織成 `map[string]interface{}` 格式。

示例：
```go
knowledge := map[string]interface{}{
    "database": "your_db",
    "tables": []interface{}{
        map[string]interface{}{
            "name": "users",
            "description": "主要使用者表",
            "columns": []string{"id", "name", "email", "birth_date"},
            "row_count": 50000,
            "stats": map[string]interface{}{
                "birth_month_distribution": map[string]int{
                    "1": 4500, "2": 4200, // ... 每月人數
                },
            },
        },
    },
}
```

### 3. 存儲到 KnowledgeManager
```go
km, _ := vectorstore.NewKnowledgeManager(cfg)
err := km.StorePhaseKnowledge("user_data", knowledge)
```

### 4. 整合 LLM 搜尋
- 檢索相關知識：`km.RetrievePhaseKnowledge("user_data", "user birthday month distribution", 5)`
- 將結果作為上下文傳給 LLM 生成回答。

## 處理狀態列表

對於寫死狀態欄位（如 status: 'pending', 'blocked', 'actived', 'deleted'），自動檢測和存儲：

### 自動檢測
在 phase1 中添加檢測邏輯：
```go
func (p *Phase1Runner) detectStatusColumns(tableName string, columns []ColumnInfo) map[string][]string {
    statusColumns := make(map[string][]string)
    for _, col := range columns {
        if strings.Contains(strings.ToLower(col.Name), "status") {
            // 查詢唯一值
            query := fmt.Sprintf("SELECT DISTINCT %s FROM %s LIMIT 100", col.Name, tableName)
            // ... 處理結果
            if len(values) <= 20 {
                statusColumns[col.Name] = values
            }
        }
    }
    return statusColumns
}
```

### 存儲
在表格分析中添加 `status_columns` 字段。

## 處理空表格

- **自動忽略**: 在 phase1 中檢查 `row_count == 0`，標記為 "default ignored"，從知識庫中排除。
- **原因**: 避免混淆 LLM，只存儲有意義的數據。

## 最佳實踐

- **分塊大小**: 調整 `ChunkSize` 避免嵌入過長。
- **元數據豐富**: 添加描述和統計提升搜尋精準度。
- **定期更新**: 數據變化時重新存儲。
- **效能考慮**: 只對小表格進行 DISTINCT 查詢。
- **測試**: 先存儲小量數據測試查詢。

## 示例使用場景

1. 存儲使用者註冊資訊的統計。
2. 查詢“使用者的生日月份分布”時，檢索相關統計，LLM 生成回答。
3. 自動檢測狀態欄位，讓 LLM 理解可能值。

此指南確保知識庫支援高效的自然語言查詢。</content>
<filePath>/Users/masato/Dev/app/go/aika-dba/docs/llm_knowledge_base_guide.md