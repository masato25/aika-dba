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

	// 將知識轉換為文本
	knowledgeText := km.knowledgeToText(phase, knowledge)

	// 分塊知識
	chunks := km.chunker.chunkText(knowledgeText, fmt.Sprintf("phase_%s", phase))

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

// knowledgeToText 將知識對象轉換為文本
func (km *KnowledgeManager) knowledgeToText(phase string, knowledge map[string]interface{}) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Phase: %s\n", phase))
	builder.WriteString(fmt.Sprintf("Description: %s\n", km.getPhaseDescription(phase)))
	builder.WriteString(fmt.Sprintf("Timestamp: %v\n\n", time.Now()))

	km.appendKnowledgeRecursive(&builder, knowledge, 0)

	return builder.String()
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
		"phase3": "Data preprocessing and transformation preparation",
		"phase4": "Dimensional modeling with star schema design and ETL planning",
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