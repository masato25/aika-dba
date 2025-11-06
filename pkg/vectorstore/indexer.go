package vectorstore

import (
	"fmt"
	"log"
	"path/filepath"
)

// KnowledgeIndexer 知識庫索引器
type KnowledgeIndexer struct {
	vectorStore *VectorStore
	embedder    Embedder
	chunker     *KnowledgeChunker
}

// NewKnowledgeIndexer 創建知識庫索引器
func NewKnowledgeIndexer(dbPath string, embedder Embedder) (*KnowledgeIndexer, error) {
	vectorStore, err := NewVectorStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %v", err)
	}

	chunker := NewKnowledgeChunker(1000, 200) // 1000行塊大小，200行重疊

	return &KnowledgeIndexer{
		vectorStore: vectorStore,
		embedder:    embedder,
		chunker:     chunker,
	}, nil
}

// IndexKnowledgeBase 索引整個知識庫
func (ki *KnowledgeIndexer) IndexKnowledgeBase(knowledgeDir string) error {
	log.Printf("Starting knowledge base indexing from: %s", knowledgeDir)

	// 清空現有數據
	if err := ki.vectorStore.Clear(); err != nil {
		return fmt.Errorf("failed to clear vector store: %v", err)
	}

	// 分塊知識庫
	chunks, err := ki.chunker.ChunkKnowledgeBase(knowledgeDir)
	if err != nil {
		return fmt.Errorf("failed to chunk knowledge base: %v", err)
	}

	log.Printf("Generated %d knowledge chunks", len(chunks))

	// 為每個塊生成嵌入並存儲
	processed := 0
	for _, chunk := range chunks {
		// 生成嵌入
		vector, err := ki.embedder.GenerateEmbedding(chunk.Content)
		if err != nil {
			log.Printf("Warning: Failed to generate embedding for chunk: %v", err)
			continue
		}

		// 存儲到向量數據庫
		if err := ki.vectorStore.AddChunk(chunk.Content, chunk.Metadata, vector); err != nil {
			log.Printf("Warning: Failed to store chunk: %v", err)
			continue
		}

		processed++
		if processed%10 == 0 {
			log.Printf("Processed %d/%d chunks", processed, len(chunks))
		}
	}

	log.Printf("Successfully indexed %d knowledge chunks", processed)
	return nil
}

// SearchKnowledge 搜索相關知識
func (ki *KnowledgeIndexer) SearchKnowledge(query string, limit int) ([]KnowledgeResult, error) {
	// 生成查詢嵌入
	queryVector, err := ki.embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// 搜索相似向量
	chunks, err := ki.vectorStore.SearchSimilar(queryVector, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar chunks: %v", err)
	}

	// 轉換為知識結果
	var results []KnowledgeResult
	for _, chunk := range chunks {
		results = append(results, KnowledgeResult{
			Content:  chunk.Content,
			Metadata: chunk.Metadata,
			Score:    0.0, // 相似度分數已經在搜索中考慮
		})
	}

	return results, nil
}

// KnowledgeResult 知識搜索結果
type KnowledgeResult struct {
	Content  string
	Metadata map[string]interface{}
	Score    float64
}

// Close 關閉索引器
func (ki *KnowledgeIndexer) Close() error {
	return ki.vectorStore.Close()
}

// GetStats 獲取索引統計信息
func (ki *KnowledgeIndexer) GetStats() (map[string]interface{}, error) {
	chunks, err := ki.vectorStore.GetAllChunks()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_chunks": len(chunks),
		"sources":      make(map[string]int),
	}

	sources := stats["sources"].(map[string]int)
	for _, chunk := range chunks {
		if source, ok := chunk.Metadata["source"].(string); ok {
			sources[source]++
		}
	}

	return stats, nil
}

// RebuildIndex 重建索引
func (ki *KnowledgeIndexer) RebuildIndex(knowledgeDir string) error {
	log.Println("Rebuilding knowledge base index...")

	if err := ki.IndexKnowledgeBase(knowledgeDir); err != nil {
		return err
	}

	log.Println("Knowledge base index rebuilt successfully")
	return nil
}

// UpdateIndex 更新索引（增量更新）
func (ki *KnowledgeIndexer) UpdateIndex(knowledgeDir string) error {
	log.Println("Updating knowledge base index...")

	// 檢查哪些文件有變化（簡單實現，實際應該檢查文件修改時間）
	files := []string{
		filepath.Join(knowledgeDir, "phase1_analysis.json"),
		filepath.Join(knowledgeDir, "phase2_analysis.json"),
		filepath.Join(knowledgeDir, "pre_phase3_summary.json"),
	}

	for _, file := range files {
		if chunks, err := ki.chunker.ChunkBySections(file, filepath.Base(file)); err == nil {
			for _, chunk := range chunks {
				vector, err := ki.embedder.GenerateEmbedding(chunk.Content)
				if err != nil {
					continue
				}

				// 檢查是否已存在（簡單實現）
				existing, _ := ki.vectorStore.SearchSimilar(vector, 1)
				if len(existing) == 0 {
					ki.vectorStore.AddChunk(chunk.Content, chunk.Metadata, vector)
				}
			}
		}
	}

	log.Println("Knowledge base index updated successfully")
	return nil
}
