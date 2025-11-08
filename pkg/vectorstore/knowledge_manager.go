package vectorstore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/masato25/aika-dba/config"
)

// KnowledgeManager 知識管理器 - 統一管理所有 phase 的向量知識
type KnowledgeManager struct {
	vectorStore *VectorStore
	embedder    Embedder
	chunker     *KnowledgeChunker
	config      *config.Config
}

// NewKnowledgeManager 創建知識管理器
func NewKnowledgeManager(cfg *config.Config) (*KnowledgeManager, error) {
	// 創建嵌入生成器
	var embedder Embedder
	switch cfg.VectorStore.EmbedderType {
	case "qwen":
		embedder = NewQwenEmbedder(cfg.VectorStore.QwenModelPath, cfg.VectorStore.EmbeddingDimension)
	case "simple":
		embedder = NewSimpleHashEmbedder(cfg.VectorStore.EmbeddingDimension)
	case "openai":
		// 從 LLM 配置中獲取 OpenAI 設定
		apiKey := cfg.LLM.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		baseURL := cfg.LLM.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := cfg.LLM.Model
		if model == "" {
			model = os.Getenv("LLM_MODEL")
		}
		// 對於OpenAI embedding，使用專門的embedding模型而不是LLM模型
		if model == "gpt-4o" || model == "gpt-4" || model == "gpt-3.5-turbo" {
			model = "text-embedding-3-small"
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		embedder = NewOpenAIEmbedder(apiKey, baseURL, model)
	default:
		embedder = NewSimpleHashEmbedder(cfg.VectorStore.EmbeddingDimension)
	}

	// 創建向量存儲
	vectorStore, err := NewVectorStore(cfg.VectorStore.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %v", err)
	}

	// 創建分塊器
	chunker := NewKnowledgeChunker(cfg.VectorStore.ChunkSize, cfg.VectorStore.ChunkOverlap)

	return &KnowledgeManager{
		vectorStore: vectorStore,
		embedder:    embedder,
		chunker:     chunker,
		config:      cfg,
	}, nil
}

// StorePhaseKnowledge 存儲特定 phase 的知識
func (km *KnowledgeManager) StorePhaseKnowledge(phase string, knowledge map[string]interface{}) error {
	log.Printf("Storing knowledge for phase: %s", phase)

	// 根據 phase 使用不同的分塊策略
	var chunks []KnowledgeChunk
	var err error

	if phase == "phase1" {
		// 對於 phase1，使用專門的表分塊邏輯
		chunks, err = km.chunkTables(knowledge, fmt.Sprintf("phase_%s", phase))
		if err != nil {
			log.Printf("Warning: Failed to chunk tables for phase1, falling back to text chunking: %v", err)
			// 回退到文本分塊
			knowledgeText := km.knowledgeToText(phase, knowledge)
			chunks = km.chunkTextSmart(knowledgeText, fmt.Sprintf("phase_%s", phase))
		}
	} else {
		// 對於其他 phase，使用智能文本分塊
		knowledgeText := km.knowledgeToText(phase, knowledge)
		chunks = km.chunkTextSmart(knowledgeText, fmt.Sprintf("phase_%s", phase))
	}

	// 存儲每個塊
	for _, chunk := range chunks {
		vector, err := km.embedder.GenerateEmbedding(chunk.Content)
		if err != nil {
			log.Printf("Warning: Failed to generate embedding for chunk: %v", err)
			continue
		}

		// 添加 phase 信息到元數據
		if chunk.Metadata == nil {
			chunk.Metadata = make(map[string]interface{})
		}
		chunk.Metadata["phase"] = phase
		chunk.Metadata["timestamp"] = time.Now().Unix()

		if err := km.vectorStore.AddChunk(chunk.Content, chunk.Metadata, vector); err != nil {
			log.Printf("Warning: Failed to store chunk: %v", err)
			continue
		}
	}

	log.Printf("Successfully stored %d knowledge chunks for phase %s", len(chunks), phase)
	return nil
}

// RetrievePhaseKnowledge 檢索特定 phase 的知識
func (km *KnowledgeManager) RetrievePhaseKnowledge(phase string, query string, limit int) ([]KnowledgeResult, error) {
	// 生成查詢向量
	queryVector, err := km.embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// 搜索所有向量，但過濾特定 phase
	allChunks, err := km.vectorStore.GetAllChunks()
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %v", err)
	}

	// 過濾特定 phase 的塊
	var phaseChunks []VectorChunk
	for _, chunk := range allChunks {
		if chunk.Metadata != nil {
			if chunkPhase, ok := chunk.Metadata["phase"].(string); ok && chunkPhase == phase {
				phaseChunks = append(phaseChunks, chunk)
			}
		}
	}

	// 計算相似度並排序
	type scoredChunk struct {
		chunk VectorChunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range phaseChunks {
		score := cosineSimilarity(queryVector, chunk.Vector)
		scored = append(scored, scoredChunk{chunk: chunk, score: score})
	}

	// 按相似度降序排序
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score < scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 返回前limit個結果
	var results []KnowledgeResult
	for i := 0; i < len(scored) && i < limit; i++ {
		results = append(results, KnowledgeResult{
			Content:  scored[i].chunk.Content,
			Metadata: scored[i].chunk.Metadata,
			Score:    scored[i].score,
		})
	}

	return results, nil
}

// RetrieveCrossPhaseKnowledge 檢索跨 phase 的知識
func (km *KnowledgeManager) RetrieveCrossPhaseKnowledge(query string, phases []string, limit int) ([]KnowledgeResult, error) {
	// 生成查詢向量
	queryVector, err := km.embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// 獲取所有塊
	allChunks, err := km.vectorStore.GetAllChunks()
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %v", err)
	}

	// 過濾指定 phase 的塊
	var filteredChunks []VectorChunk
	for _, chunk := range allChunks {
		if chunk.Metadata != nil {
			if chunkPhase, ok := chunk.Metadata["phase"].(string); ok {
				for _, targetPhase := range phases {
					if chunkPhase == targetPhase {
						filteredChunks = append(filteredChunks, chunk)
						break
					}
				}
			}
		}
	}

	// 計算相似度並排序
	type scoredChunk struct {
		chunk VectorChunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range filteredChunks {
		score := cosineSimilarity(queryVector, chunk.Vector)
		scored = append(scored, scoredChunk{chunk: chunk, score: score})
	}

	// 按相似度降序排序
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score < scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 返回前limit個結果
	var results []KnowledgeResult
	for i := 0; i < len(scored) && i < limit; i++ {
		results = append(results, KnowledgeResult{
			Content:  scored[i].chunk.Content,
			Metadata: scored[i].chunk.Metadata,
			Score:    scored[i].score,
		})
	}

	return results, nil
}

// chunkTables 專門處理表結構分塊
func (km *KnowledgeManager) chunkTables(data map[string]interface{}, source string) ([]KnowledgeChunk, error) {
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

// knowledgeToText 將知識對象轉換為文本
func (km *KnowledgeManager) knowledgeToText(phase string, knowledge map[string]interface{}) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Phase: %s\n", phase))
	builder.WriteString(fmt.Sprintf("Description: %s\n", km.getPhaseDescription(phase)))
	builder.WriteString(fmt.Sprintf("Timestamp: %v\n\n", time.Now()))

	km.appendKnowledgeRecursive(&builder, knowledge, 0)

	return builder.String()
}

// chunkTextSmart 智能文本分塊 - 按字符數而不是行數
func (km *KnowledgeManager) chunkTextSmart(text, source string) []KnowledgeChunk {
	var chunks []KnowledgeChunk

	// 使用字符數分塊而不是行數
	chunkSize := km.config.VectorStore.ChunkSize       // 1000 字符
	chunkOverlap := km.config.VectorStore.ChunkOverlap // 200 字符

	runes := []rune(text)
	textLen := len(runes)

	for i := 0; i < textLen; i += chunkSize - chunkOverlap {
		end := i + chunkSize
		if end > textLen {
			end = textLen
		}

		chunkContent := string(runes[i:end])

		// 確保塊不為空且有意義
		chunkContent = strings.TrimSpace(chunkContent)
		if chunkContent == "" {
			continue
		}

		chunk := KnowledgeChunk{
			Content: chunkContent,
			Metadata: map[string]interface{}{
				"source": source,
				"type":   "knowledge_chunk",
			},
			Source: source,
		}
		chunks = append(chunks, chunk)

		// 如果已經到達文本結尾，停止
		if end == textLen {
			break
		}
	}

	return chunks
}

// appendKnowledgeRecursive 遞歸添加知識內容
func (km *KnowledgeManager) appendKnowledgeRecursive(builder *strings.Builder, data interface{}, depth int) {
	indent := strings.Repeat("  ", depth)

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			builder.WriteString(fmt.Sprintf("%s%s:\n", indent, key))
			km.appendKnowledgeRecursive(builder, value, depth+1)
		}
	case []interface{}:
		for i, item := range v {
			builder.WriteString(fmt.Sprintf("%s[%d]:\n", indent, i))
			km.appendKnowledgeRecursive(builder, item, depth+1)
		}
	default:
		builder.WriteString(fmt.Sprintf("%s%v\n", indent, v))
	}
}

// getPhaseDescription 獲取 phase 描述
func (km *KnowledgeManager) getPhaseDescription(phase string) string {
	descriptions := map[string]string{
		"phase1": "Database statistical analysis with table schemas, constraints, and sample data",
		"phase2": "AI-powered business logic analysis with LLM insights and recommendations",
		"phase3": "Business logic analysis and natural language description generation",
	}
	return descriptions[phase]
}

// GetKnowledgeStats 獲取知識統計信息
func (km *KnowledgeManager) GetKnowledgeStats() (map[string]interface{}, error) {
	chunks, err := km.vectorStore.GetAllChunks()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_chunks": len(chunks),
		"phases":       make(map[string]int),
	}

	phases := stats["phases"].(map[string]int)
	for _, chunk := range chunks {
		if chunk.Metadata != nil {
			if phase, ok := chunk.Metadata["phase"].(string); ok {
				phases[phase]++
			}
		}
	}

	return stats, nil
}

// Close 關閉知識管理器
func (km *KnowledgeManager) Close() error {
	return km.vectorStore.Close()
}

// DeletePhaseKnowledge 刪除特定 phase 的知識
func (km *KnowledgeManager) DeletePhaseKnowledge(phase string) error {
	log.Printf("Deleting knowledge for phase: %s", phase)

	err := km.vectorStore.DeleteByMetadata("phase", phase)
	if err != nil {
		return fmt.Errorf("failed to delete phase %s knowledge: %v", phase, err)
	}

	log.Printf("Successfully deleted knowledge for phase %s", phase)
	return nil
}

// ExportKnowledgeToFile 將知識導出到文件（用於調試）

// ExportKnowledgeToFile 將知識導出到文件（用於調試）
func (km *KnowledgeManager) ExportKnowledgeToFile(filename string) error {
	chunks, err := km.vectorStore.GetAllChunks()
	if err != nil {
		return err
	}

	// 按 phase 分組
	phaseChunks := make(map[string][]VectorChunk)
	for _, chunk := range chunks {
		if chunk.Metadata != nil {
			if phase, ok := chunk.Metadata["phase"].(string); ok {
				phaseChunks[phase] = append(phaseChunks[phase], chunk)
			}
		}
	}

	// 導出到文件
	exportData := map[string]interface{}{
		"export_timestamp": time.Now(),
		"total_chunks":     len(chunks),
		"phases":           phaseChunks,
	}

	return km.exportToJSONFile(exportData, filename)
}

// exportToJSONFile 導出到 JSON 文件
func (km *KnowledgeManager) exportToJSONFile(data interface{}, filename string) error {
	// 確保目錄存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
