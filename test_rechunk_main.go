package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

func main() {
	// 載入配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 臨時修改配置使用簡單embedder來測試分塊
	cfg.VectorStore.EmbedderType = "simple"

	// 創建知識管理器
	km, err := vectorstore.NewKnowledgeManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create knowledge manager: %v", err)
	}
	defer km.Close()

	// 清除現有知識
	fmt.Println("Clearing existing knowledge...")
	phases := []string{"phase1", "phase2", "phase3"}
	for _, phase := range phases {
		err := km.DeletePhaseKnowledge(phase)
		if err != nil {
			log.Printf("Warning: Failed to delete phase %s: %v", phase, err)
		}
	}

	// 重新存儲知識
	fmt.Println("Re-storing knowledge with improved chunking...")

	// Phase 1: 載入並存儲 phase1 知識
	phase1File := "knowledge/phase1_analysis.json"
	if _, err := os.Stat(phase1File); err == nil {
		data, err := os.ReadFile(phase1File)
		if err == nil {
			var knowledge map[string]interface{}
			if err := json.Unmarshal(data, &knowledge); err == nil {
				err := km.StorePhaseKnowledge("phase1", knowledge)
				if err != nil {
					log.Printf("Failed to store phase1: %v", err)
				} else {
					fmt.Println("Successfully stored phase1 knowledge")
				}
			}
		}
	}

	// Phase 2: 載入並存儲 phase2 知識
	phase2File := "knowledge/phase2_analysis.json"
	if _, err := os.Stat(phase2File); err == nil {
		data, err := os.ReadFile(phase2File)
		if err == nil {
			var knowledge map[string]interface{}
			if err := json.Unmarshal(data, &knowledge); err == nil {
				err := km.StorePhaseKnowledge("phase2", knowledge)
				if err != nil {
					log.Printf("Failed to store phase2: %v", err)
				} else {
					fmt.Println("Successfully stored phase2 knowledge")
				}
			}
		}
	}

	// Phase 3: 載入並存儲 phase3 知識
	phase3File := "knowledge/phase3_analysis.json"
	if _, err := os.Stat(phase3File); err == nil {
		data, err := os.ReadFile(phase3File)
		if err == nil {
			var knowledge map[string]interface{}
			if err := json.Unmarshal(data, &knowledge); err == nil {
				err := km.StorePhaseKnowledge("phase3", knowledge)
				if err != nil {
					log.Printf("Failed to store phase3: %v", err)
				} else {
					fmt.Println("Successfully stored phase3 knowledge")
				}
			}
		}
	}

	// 獲取統計信息
	stats, err := km.GetKnowledgeStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		fmt.Printf("\nKnowledge Statistics:\n")
		fmt.Printf("Total chunks: %v\n", stats["total_chunks"])
		fmt.Printf("Phases: %v\n", stats["phases"])
	}

	// 導出知識以供檢查
	err = km.ExportKnowledgeToFile("debug_chunks_improved.json")
	if err != nil {
		log.Printf("Failed to export knowledge: %v", err)
	} else {
		fmt.Println("Exported knowledge to debug_chunks_improved.json")
	}
}
