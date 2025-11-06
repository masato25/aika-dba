package phases

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// Phase1PutRunner Phase 1 Put 執行器 - 根據 post 分析結果更新 phase1
type Phase1PutRunner struct {
	config       *config.Config
	knowledgeMgr *vectorstore.KnowledgeManager
}

// NewPhase1PutRunner 創建 Phase 1 Put 執行器
func NewPhase1PutRunner(cfg *config.Config) (*Phase1PutRunner, error) {
	// 創建知識管理器
	knowledgeMgr, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		return nil, err
	}

	return &Phase1PutRunner{
		config:       cfg,
		knowledgeMgr: knowledgeMgr,
	}, nil
}

// Run 執行 Phase 1 Put - 更新 phase1 結果
func (p *Phase1PutRunner) Run() error {
	log.Println("=== Starting Phase 1 Put: Update Phase 1 Results Based on Post Analysis ===")

	// 讀取原始 phase1 分析結果
	phase1Data, err := p.loadPhase1Results()
	if err != nil {
		return fmt.Errorf("failed to load phase1 results: %v", err)
	}

	// 讀取 phase1_post 分析結果
	postData, err := p.loadPhase1PostResults()
	if err != nil {
		return fmt.Errorf("failed to load phase1_post results: %v", err)
	}

	// 根據 post 決策過濾表格
	filteredData, excludedTables := p.filterTablesBasedOnDecisions(phase1Data, postData)

	// 更新時間戳
	filteredData["timestamp"] = time.Now()
	filteredData["phase1_put_applied"] = true
	filteredData["excluded_tables"] = excludedTables
	filteredData["excluded_count"] = len(excludedTables)

	// 寫入更新後的 phase1 結果
	if err := p.writeOutput(filteredData, "knowledge/phase1_analysis.json"); err != nil {
		return fmt.Errorf("failed to write updated phase1 results: %v", err)
	}

	// 更新向量存儲
	if err := p.updateVectorStore(filteredData); err != nil {
		log.Printf("Warning: Failed to update vector store: %v", err)
	} else {
		log.Printf("Vector store updated with filtered phase1 results")
	}

	log.Printf("Phase 1 Put completed. Excluded %d tables, kept %d tables", len(excludedTables), len(filteredData["tables"].(map[string]interface{})))
	
	if len(excludedTables) > 0 {
		log.Printf("Excluded tables: %v", excludedTables)
	}

	return nil
}

// loadPhase1Results 讀取 Phase 1 的分析結果
func (p *Phase1PutRunner) loadPhase1Results() (map[string]interface{}, error) {
	file, err := os.Open("knowledge/phase1_analysis.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// loadPhase1PostResults 讀取 Phase 1 Post 的分析結果
func (p *Phase1PutRunner) loadPhase1PostResults() (map[string]interface{}, error) {
	file, err := os.Open("knowledge/phase1_post_analysis.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// filterTablesBasedOnDecisions 根據 post 決策過濾表格
func (p *Phase1PutRunner) filterTablesBasedOnDecisions(phase1Data, postData map[string]interface{}) (map[string]interface{}, []string) {
	// 創建過濾後的數據副本
	filteredData := make(map[string]interface{})
	for k, v := range phase1Data {
		filteredData[k] = v
	}

	// 提取要刪除的表格列表
	decisions, ok := postData["decisions"].(map[string]interface{})
	if !ok {
		log.Printf("Warning: No decisions found in phase1_post data")
		return filteredData, []string{}
	}

	summary, ok := decisions["summary"].(map[string]interface{})
	if !ok {
		log.Printf("Warning: No summary found in decisions")
		return filteredData, []string{}
	}

	tablesToDrop, ok := summary["tables_to_drop"].([]interface{})
	if !ok {
		log.Printf("Warning: No tables_to_drop found in summary")
		return filteredData, []string{}
	}

	// 創建要刪除的表格集合
	tablesToDropSet := make(map[string]bool)
	var excludedTables []string
	for _, table := range tablesToDrop {
		if tableName, ok := table.(string); ok {
			tablesToDropSet[tableName] = true
			excludedTables = append(excludedTables, tableName)
		}
	}

	// 過濾表格數據
	tables, ok := filteredData["tables"].(map[string]interface{})
	if !ok {
		log.Printf("Warning: No tables data found in phase1 results")
		return filteredData, excludedTables
	}

	filteredTables := make(map[string]interface{})
	for tableName, tableData := range tables {
		if !tablesToDropSet[tableName] {
			filteredTables[tableName] = tableData
		}
	}

	filteredData["tables"] = filteredTables
	filteredData["tables_count"] = len(filteredTables)

	return filteredData, excludedTables
}

// updateVectorStore 更新向量存儲
func (p *Phase1PutRunner) updateVectorStore(data map[string]interface{}) error {
	// 首先清除舊的 phase1 知識
	if err := p.knowledgeMgr.DeletePhaseKnowledge("phase1"); err != nil {
		log.Printf("Warning: Failed to delete old phase1 knowledge: %v", err)
	}

	// 存儲新的 phase1 知識
	if err := p.knowledgeMgr.StorePhaseKnowledge("phase1", data); err != nil {
		return fmt.Errorf("failed to store updated phase1 knowledge: %w", err)
	}

	return nil
}

// writeOutput 寫入輸出到文件
func (p *Phase1PutRunner) writeOutput(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	log.Printf("Updated phase1 results saved to %s", filename)
	return nil
}

// Close 關閉 Phase 1 Put 執行器
func (p *Phase1PutRunner) Close() error {
	if p.knowledgeMgr != nil {
		return p.knowledgeMgr.Close()
	}
	return nil
}
