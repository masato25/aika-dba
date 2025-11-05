package vectorstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Embedder 嵌入生成器接口
type Embedder interface {
	GenerateEmbedding(text string) ([]float64, error)
}

// SimpleHashEmbedder 簡單的哈希嵌入生成器
type SimpleHashEmbedder struct {
	dimension int
}

// NewSimpleHashEmbedder 創建簡單哈希嵌入生成器
func NewSimpleHashEmbedder(dimension int) *SimpleHashEmbedder {
	return &SimpleHashEmbedder{dimension: dimension}
}

// GenerateEmbedding 生成基於哈希的嵌入向量
func (e *SimpleHashEmbedder) GenerateEmbedding(text string) ([]float64, error) {
	// 使用SHA256生成哈希
	hash := sha256.Sum256([]byte(text))
	hashStr := hex.EncodeToString(hash[:])

	// 將哈希轉換為浮點數向量
	vector := make([]float64, e.dimension)
	for i := 0; i < e.dimension && i*8 < len(hashStr); i++ {
		// 每8個字符轉換為一個64位浮點數
		start := i * 8
		end := start + 8
		if end > len(hashStr) {
			end = len(hashStr)
		}

		// 將十六進制字符串轉換為整數，然後轉為浮點數
		if val, err := strconv.ParseUint(hashStr[start:end], 16, 64); err == nil {
			vector[i] = float64(val) / float64(1<<64) // 正規化到[0,1)
		} else {
			vector[i] = 0.0
		}
	}

	// 正規化向量
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector, nil
}

// LLMEmbedder 使用LLM服務生成嵌入
type LLMEmbedder struct {
	host   string
	port   int
	model  string
	client *http.Client
}

// NewLLMEmbedder 創建LLM嵌入生成器
func NewLLMEmbedder(host string, port int, model string) *LLMEmbedder {
	return &LLMEmbedder{
		host:   host,
		port:   port,
		model:  model,
		client: &http.Client{},
	}
}

// GenerateEmbedding 使用LLM生成嵌入向量
func (e *LLMEmbedder) GenerateEmbedding(text string) ([]float64, error) {
	// 對於簡單實現，我們使用文本長度和字符頻率來生成向量
	// 在實際應用中，這裡應該調用LLM的嵌入API

	vector := make([]float64, 384) // 使用384維向量

	// 基於文本統計的簡單嵌入
	vector[0] = float64(len(text)) / 1000.0                            // 文本長度
	vector[1] = float64(strings.Count(text, " ")) / float64(len(text)) // 空格密度
	vector[2] = float64(strings.Count(text, ".")) / float64(len(text)) // 句點密度
	vector[3] = float64(strings.Count(text, ",")) / float64(len(text)) // 逗號密度

	// 計算字符頻率
	charCount := make(map[rune]int)
	for _, r := range text {
		charCount[r]++
	}

	// 使用前幾個字符的頻率
	i := 4
	for r := 'a'; r <= 'z' && i < len(vector); r++ {
		vector[i] = float64(charCount[r]) / float64(len(text))
		i++
	}

	for r := 'A'; r <= 'Z' && i < len(vector); r++ {
		vector[i] = float64(charCount[r]) / float64(len(text))
		i++
	}

	// 正規化向量
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for j := range vector {
			vector[j] /= norm
		}
	}

	return vector, nil
}

// GenerateEmbeddingWithLLM 使用LLM API生成嵌入（未實現，需要具體的嵌入API）
func (e *LLMEmbedder) GenerateEmbeddingWithLLM(text string) ([]float64, error) {
	// 這裡應該實現對LLM嵌入API的調用
	// 例如：OpenAI的embeddings API或其他本地LLM的嵌入端點

	ctx := context.Background()

	requestBody := map[string]interface{}{
		"model": e.model,
		"input": text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d/v1/embeddings", e.host, e.port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// 解析響應中的嵌入向量
	// 這裡需要根據具體的API響應格式來調整
	data, ok := response["data"].([]interface{})
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("invalid embedding response format")
	}

	embeddingData, ok := data[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid embedding data format")
	}

	embedding, ok := embeddingData["embedding"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no embedding found in response")
	}

	vector := make([]float64, len(embedding))
	for i, v := range embedding {
		if val, ok := v.(float64); ok {
			vector[i] = val
		}
	}

	return vector, nil
}

// QwenEmbedder 使用改進的文本嵌入生成器
type QwenEmbedder struct {
	dimension int
	vocab     map[string]int     // 詞彙表
	idfScores map[string]float64 // IDF 分數
}

// NewQwenEmbedder 創建改進的嵌入生成器
func NewQwenEmbedder(modelPath string, dimension int) *QwenEmbedder {
	embedder := &QwenEmbedder{
		dimension: dimension,
		vocab:     make(map[string]int),
		idfScores: make(map[string]float64),
	}

	// 初始化詞彙表和 IDF 分數（基於常見的技術術語）
	embedder.initializeVocabulary()

	return embedder
}

// initializeVocabulary 初始化詞彙表
func (qe *QwenEmbedder) initializeVocabulary() {
	// 資料庫相關術語
	dbTerms := []string{
		"table", "column", "row", "database", "sql", "query", "select", "insert", "update", "delete",
		"primary", "foreign", "key", "index", "constraint", "join", "where", "group", "order", "limit",
		"customer", "product", "order", "sale", "revenue", "price", "quantity", "total", "date", "time",
		"analysis", "report", "metric", "trend", "performance", "efficiency", "optimization",
	}

	// 行銷相關術語
	marketingTerms := []string{
		"marketing", "sales", "customer", "segment", "campaign", "conversion", "retention", "churn",
		"acquisition", "lifetime", "value", "cohort", "funnel", "engagement", "loyalty", "brand",
		"channel", "roi", "kpi", "metric", "analytics", "insight", "strategy", "growth",
	}

	allTerms := append(dbTerms, marketingTerms...)

	// 為每個術語分配索引和 IDF 分數
	for i, term := range allTerms {
		qe.vocab[term] = i
		// 簡單的 IDF 計算（較常見的詞有較低的 IDF）
		if strings.Contains(term, "customer") || strings.Contains(term, "product") {
			qe.idfScores[term] = 1.0 // 常見詞
		} else {
			qe.idfScores[term] = 2.0 // 不常見詞
		}
	}
}

// GenerateEmbedding 生成改進的文本嵌入
func (qe *QwenEmbedder) GenerateEmbedding(text string) ([]float64, error) {
	// 預處理文本
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, ".", " ")
	text = strings.ReplaceAll(text, ",", " ")
	text = strings.ReplaceAll(text, ":", " ")
	text = strings.ReplaceAll(text, ";", " ")
	text = strings.ReplaceAll(text, "!", " ")
	text = strings.ReplaceAll(text, "?", " ")

	words := strings.Fields(text)

	// 計算 TF-IDF 向量
	vector := make([]float64, qe.dimension)

	// 計算詞頻
	wordCount := make(map[string]int)
	totalWords := 0

	for _, word := range words {
		if _, exists := qe.vocab[word]; exists {
			wordCount[word]++
			totalWords++
		}
	}

	// 生成 TF-IDF 向量
	for word, count := range wordCount {
		if vocabIndex, exists := qe.vocab[word]; exists && vocabIndex < qe.dimension {
			tf := float64(count) / float64(totalWords)
			idf := qe.idfScores[word]
			vector[vocabIndex] = tf * idf
		}
	}

	// 添加位置編碼（簡單版本）
	for i := range vector {
		if i < len(words) {
			vector[i] += 0.1 * float64(i) / float64(len(words))
		}
	}

	// 添加文本統計特徵
	vector = qe.addTextFeatures(vector, text, words)

	// 正規化向量
	qe.normalizeVector(vector)

	return vector, nil
}

// addTextFeatures 添加文本統計特徵
func (qe *QwenEmbedder) addTextFeatures(vector []float64, text string, words []string) []float64 {
	// 如果向量不夠大，返回原向量
	if len(vector) < 10 {
		return vector
	}

	// 添加基本統計特徵到向量的最後幾個位置
	statsIndex := len(vector) - 10

	vector[statsIndex] = float64(len(text)) / 1000.0                  // 文本長度
	vector[statsIndex+1] = float64(len(words)) / 100.0                // 詞數
	vector[statsIndex+2] = float64(len(text)) / float64(len(words)+1) // 平均詞長

	// 計算詞彙多樣性
	uniqueWords := make(map[string]bool)
	for _, word := range words {
		uniqueWords[word] = true
	}
	vector[statsIndex+3] = float64(len(uniqueWords)) / float64(len(words)+1) // 詞彙多樣性

	// 簡單的語義特徵
	vector[statsIndex+4] = float64(strings.Count(text, "分析")) / 10.0
	vector[statsIndex+5] = float64(strings.Count(text, "查詢")) / 10.0
	vector[statsIndex+6] = float64(strings.Count(text, "銷售")) / 10.0
	vector[statsIndex+7] = float64(strings.Count(text, "客戶")) / 10.0
	vector[statsIndex+8] = float64(strings.Count(text, "產品")) / 10.0
	vector[statsIndex+9] = float64(strings.Count(text, "數據")) / 10.0

	return vector
}

// normalizeVector 正規化向量
func (qe *QwenEmbedder) normalizeVector(vector []float64) {
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}
}

// Close 關閉嵌入生成器（無操作）
func (qe *QwenEmbedder) Close() error {
	return nil
}
