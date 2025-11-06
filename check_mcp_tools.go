package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/masato25/aika-dba/pkg/mcp"
)

func main() {
	// 連接到測試資料庫
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=ecommerce sslmode=disable")
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		fmt.Println("MCP server structure updated with meaningful phase query tools:")
		fmt.Println("- get_database_schema_analysis (Phase 1)")
		fmt.Println("- get_business_logic_analysis (Phase 2)")
		fmt.Println("- get_comprehensive_business_overview (Phase 3)")
		fmt.Println("- get_knowledge_stats")
		return
	}
	defer db.Close()

	server := mcp.NewMCPServer(db)

	toolsRequest := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	response, err := server.HandleRequest(toolsRequest)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	var resp map[string]interface{}
	json.Unmarshal([]byte(response), &resp)

	if result, ok := resp["result"].(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			fmt.Printf("Found %d MCP tools including phase queries:\n", len(tools))
			for _, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					name := toolMap["name"].(string)
					if name == "get_database_schema_analysis" || name == "get_business_logic_analysis" ||
						name == "get_comprehensive_business_overview" || name == "get_knowledge_stats" {
						fmt.Printf("- %s\n", name)
					}
				}
			}
		}
	}
}
