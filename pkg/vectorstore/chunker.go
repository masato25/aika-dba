package vectorstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KnowledgeChunker 知識庫分塊器
type KnowledgeChunker struct {
	chunkSize    int
	chunkOverlap int
}

// NewKnowledgeChunker 創建知識庫分塊器
func NewKnowledgeChunker(chunkSize, chunkOverlap int) *KnowledgeChunker {
	return &KnowledgeChunker{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// ChunkKnowledgeBase 將知識庫文件分塊
func (kc *KnowledgeChunker) ChunkKnowledgeBase(knowledgeDir string) ([]KnowledgeChunk, error) {
	var allChunks []KnowledgeChunk

	// 處理所有JSON文件
	files := []string{
		"phase1_analysis.json",
		"phase2_analysis.json",
		"pre_phase3_summary.json",
	}

	for _, filename := range files {
		filePath := filepath.Join(knowledgeDir, filename)
		chunks, err := kc.ChunkBySections(filePath, filename)
		if err != nil {
			// 記錄警告但繼續處理其他文件
			fmt.Printf("Warning: Failed to chunk file %s: %v\n", filename, err)
			continue
		}
		allChunks = append(allChunks, chunks...)
	}

	return allChunks, nil
}

// KnowledgeChunk 知識塊結構
type KnowledgeChunk struct {
	Content  string
	Metadata map[string]interface{}
	Source   string
}

// chunkFile 將單個文件分塊 (已棄用，由 ChunkBySections 替代)
func (kc *KnowledgeChunker) chunkFile(filePath, source string) ([]KnowledgeChunk, error) {
	// 讀取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// 解析JSON
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// 將JSON轉換為文本表示
	content := kc.jsonToText(jsonData)

	// 分塊
	return kc.chunkText(content, source), nil
}

// jsonToText 將JSON數據轉換為文本 (已棄用，由結構化處理替代)
func (kc *KnowledgeChunker) jsonToText(data interface{}) string {
	var builder strings.Builder

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			builder.WriteString(fmt.Sprintf("%s: ", key))
			builder.WriteString(kc.jsonToText(value))
			builder.WriteString("\n")
		}
	case []interface{}:
		for i, item := range v {
			builder.WriteString(fmt.Sprintf("[%d]: ", i))
			builder.WriteString(kc.jsonToText(item))
			builder.WriteString("\n")
		}
	default:
		builder.WriteString(fmt.Sprintf("%v", v))
	}

	return builder.String()
}

// chunkText 將文本分塊
func (kc *KnowledgeChunker) chunkText(text, source string) []KnowledgeChunk {
	var chunks []KnowledgeChunk

	// 按行分割文本
	lines := strings.Split(text, "\n")

	currentChunk := ""
	lineCount := 0

	for _, line := range lines {
		// 如果添加這行會超過塊大小，保存當前塊
		if lineCount+1 > kc.chunkSize && currentChunk != "" {
			chunk := KnowledgeChunk{
				Content: strings.TrimSpace(currentChunk),
				Metadata: map[string]interface{}{
					"source": source,
					"type":   "knowledge_chunk",
				},
				Source: source,
			}
			if chunk.Content != "" {
				chunks = append(chunks, chunk)
			}

			// 開始新塊，保留一些重疊
			overlapLines := kc.getOverlapLines(currentChunk)
			currentChunk = overlapLines + line
			lineCount = strings.Count(overlapLines, "\n") + 1
		} else {
			if currentChunk != "" {
				currentChunk += "\n"
			}
			currentChunk += line
			lineCount++
		}
	}

	// 添加最後一個塊
	if currentChunk != "" {
		chunk := KnowledgeChunk{
			Content: strings.TrimSpace(currentChunk),
			Metadata: map[string]interface{}{
				"source": source,
				"type":   "knowledge_chunk",
			},
			Source: source,
		}
		if chunk.Content != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// getOverlapLines 獲取重疊行
func (kc *KnowledgeChunker) getOverlapLines(text string) string {
	lines := strings.Split(text, "\n")
	overlapCount := kc.chunkOverlap

	if len(lines) <= overlapCount {
		return text
	}

	// 取最後幾行作為重疊
	start := len(lines) - overlapCount
	if start < 0 {
		start = 0
	}

	return strings.Join(lines[start:], "\n")
}

// ChunkBySections 按JSON結構分塊（更智能的分塊方法）
func (kc *KnowledgeChunker) ChunkBySections(filePath, source string) ([]KnowledgeChunk, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	var chunks []KnowledgeChunk

	// 對於phase1_analysis.json，特殊處理表結構
	if strings.Contains(source, "phase1") {
		fmt.Printf("Processing phase1 file: %s\n", source)
		tableChunks, err := kc.chunkTables(jsonData, source)
		if err == nil {
			fmt.Printf("Generated %d table chunks\n", len(tableChunks))
			chunks = append(chunks, tableChunks...)
		} else {
			fmt.Printf("Failed to chunk tables: %v\n", err)
		}
	} else {
		// 對於其他文件，使用通用分塊
		textContent := kc.jsonToText(jsonData)
		textChunks := kc.chunkText(textContent, source)
		chunks = append(chunks, textChunks...)
	}

	return chunks, nil
}

// chunkTables 專門處理表結構分塊
func (kc *KnowledgeChunker) chunkTables(data map[string]interface{}, source string) ([]KnowledgeChunk, error) {
	var chunks []KnowledgeChunk

	tables, ok := data["tables"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no tables found in phase1 data")
	}

	fmt.Printf("Found %d tables to process\n", len(tables))

	for tableName, tableData := range tables {
		fmt.Printf("Processing table: %s\n", tableName)
		tableInfo, ok := tableData.(map[string]interface{})
		if !ok {
			continue
		}

		// 創建表結構塊
		var content strings.Builder
		content.WriteString(fmt.Sprintf("Table: %s\n", tableName))

		// 正確處理 schema 數組
		if schemaArray, ok := tableInfo["schema"].([]interface{}); ok {
			content.WriteString("Columns:\n")
			for _, colInterface := range schemaArray {
				if colMap, ok := colInterface.(map[string]interface{}); ok {
					colName := colMap["name"]
					colType := colMap["type"]
					colNullable := colMap["nullable"]
					content.WriteString(fmt.Sprintf("  - %s (%v, nullable: %v)\n", colName, colType, colNullable))
				}
			}
		}

		// 處理約束
		if constraints, ok := tableInfo["constraints"].(map[string]interface{}); ok {
			content.WriteString("Constraints:\n")
			if pks, ok := constraints["primary_keys"].([]interface{}); ok && len(pks) > 0 {
				content.WriteString(fmt.Sprintf("  Primary Keys: %v\n", pks))
			}
			if fks, ok := constraints["foreign_keys"].([]interface{}); ok && len(fks) > 0 {
				content.WriteString("  Foreign Keys:\n")
				for _, fkInterface := range fks {
					if fkMap, ok := fkInterface.(map[string]interface{}); ok {
						content.WriteString(fmt.Sprintf("    - %s -> %s.%s\n", fkMap["column"], fkMap["referenced_table"], fkMap["referenced_column"]))
					}
				}
			}
		}

		// 處理索引
		if indexes, ok := tableInfo["indexes"].([]interface{}); ok && len(indexes) > 0 {
			content.WriteString("Indexes:\n")
			for _, idxInterface := range indexes {
				if idxMap, ok := idxInterface.(map[string]interface{}); ok {
					idxName := idxMap["name"]
					idxColumns := idxMap["columns"]
					isUnique := idxMap["is_unique"]
					content.WriteString(fmt.Sprintf("  - %s on %v (unique: %v)\n", idxName, idxColumns, isUnique))
				}
			}
		}

		chunk := KnowledgeChunk{
			Content: content.String(),
			Metadata: map[string]interface{}{
				"source": source,
				"type":   "table_schema",
				"table":  tableName,
			},
			Source: source,
		}
		chunks = append(chunks, chunk)

		// 創建樣本數據塊（如果有的話）
		if samples, ok := tableInfo["samples"].([]interface{}); ok && len(samples) > 0 {
			var sampleContent strings.Builder
			sampleContent.WriteString(fmt.Sprintf("Sample data for table: %s\n", tableName))
			for i, sample := range samples {
				if i >= 3 { // 只取前3個樣本
					break
				}
				sampleContent.WriteString(fmt.Sprintf("Sample %d: %v\n", i+1, sample))
			}

			sampleChunk := KnowledgeChunk{
				Content: sampleContent.String(),
				Metadata: map[string]interface{}{
					"source": source,
					"type":   "table_samples",
					"table":  tableName,
				},
				Source: source,
			}
			chunks = append(chunks, sampleChunk)
		}
	}

	fmt.Printf("Generated %d chunks for tables\n", len(chunks))
	return chunks, nil
}
