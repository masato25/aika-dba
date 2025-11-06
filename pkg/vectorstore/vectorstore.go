package vectorstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// VectorStore 向量存儲結構
type VectorStore struct {
	db *sql.DB
}

// VectorChunk 向量塊結構
type VectorChunk struct {
	ID       int                    `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Vector   []float64              `json:"vector"`
}

// NewVectorStore 創建新的向量存儲
func NewVectorStore(dbPath string) (*VectorStore, error) {
	// 確保目錄存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// 創建表結構
	if err := initTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %v", err)
	}

	return &VectorStore{db: db}, nil
}

// initTables 初始化數據庫表
func initTables(db *sql.DB) error {
	// 創建向量塊表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS vector_chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		metadata TEXT,
		vector TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_content ON vector_chunks(content);
	`

	_, err := db.Exec(createTableSQL)
	return err
}

// Close 關閉向量存儲
func (vs *VectorStore) Close() error {
	if vs.db != nil {
		return vs.db.Close()
	}
	return nil
}

// AddChunk 添加向量塊
func (vs *VectorStore) AddChunk(content string, metadata map[string]interface{}, vector []float64) error {
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal vector: %v", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %v", err)
	}

	_, err = vs.db.Exec(
		"INSERT INTO vector_chunks (content, metadata, vector) VALUES (?, ?, ?)",
		content, string(metadataJSON), string(vectorJSON),
	)

	return err
}

// SearchSimilar 搜索相似向量
func (vs *VectorStore) SearchSimilar(queryVector []float64, limit int) ([]VectorChunk, error) {
	rows, err := vs.db.Query("SELECT id, content, metadata, vector FROM vector_chunks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []VectorChunk
	for rows.Next() {
		var id int
		var content, metadataStr, vectorStr string

		if err := rows.Scan(&id, &content, &metadataStr, &vectorStr); err != nil {
			continue
		}

		var vector []float64
		if err := json.Unmarshal([]byte(vectorStr), &vector); err != nil {
			continue
		}

		var metadata map[string]interface{}
		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &metadata)
		}

		chunk := VectorChunk{
			ID:       id,
			Content:  content,
			Metadata: metadata,
			Vector:   vector,
		}

		chunks = append(chunks, chunk)
	}

	// 計算相似度並排序
	type scoredChunk struct {
		chunk VectorChunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range chunks {
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
	var results []VectorChunk
	for i := 0; i < len(scored) && i < limit; i++ {
		results = append(results, scored[i].chunk)
	}

	return results, nil
}

// cosineSimilarity 計算餘弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GetAllChunks 獲取所有向量塊（用於調試）
func (vs *VectorStore) GetAllChunks() ([]VectorChunk, error) {
	rows, err := vs.db.Query("SELECT id, content, metadata, vector FROM vector_chunks ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []VectorChunk
	for rows.Next() {
		var id int
		var content, metadataStr, vectorStr string

		if err := rows.Scan(&id, &content, &metadataStr, &vectorStr); err != nil {
			continue
		}

		var vector []float64
		if err := json.Unmarshal([]byte(vectorStr), &vector); err != nil {
			continue
		}

		var metadata map[string]interface{}
		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &metadata)
		}

		chunks = append(chunks, VectorChunk{
			ID:       id,
			Content:  content,
			Metadata: metadata,
			Vector:   vector,
		})
	}

	return chunks, nil
}

// Clear 清空所有向量塊
func (vs *VectorStore) Clear() error {
	_, err := vs.db.Exec("DELETE FROM vector_chunks")
	return err
}

// DeleteByMetadata 根據元數據刪除向量塊
func (vs *VectorStore) DeleteByMetadata(key string, value interface{}) error {
	// 獲取所有塊，然後過濾並刪除
	rows, err := vs.db.Query("SELECT id, metadata FROM vector_chunks")
	if err != nil {
		return err
	}
	defer rows.Close()

	var idsToDelete []int
	for rows.Next() {
		var id int
		var metadataStr string

		if err := rows.Scan(&id, &metadataStr); err != nil {
			continue
		}

		var metadata map[string]interface{}
		if metadataStr != "" {
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				continue
			}
		}

		// 檢查元數據是否匹配
		if metadata != nil {
			if metaValue, ok := metadata[key]; ok {
				if metaValue == value {
					idsToDelete = append(idsToDelete, id)
				}
			}
		}
	}

	// 刪除匹配的記錄
	for _, id := range idsToDelete {
		_, err := vs.db.Exec("DELETE FROM vector_chunks WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("failed to delete chunk %d: %v", id, err)
		}
	}

	return nil
}
