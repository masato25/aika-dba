package mcp

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/analyzer"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// MCPServer MCP 服務器
type MCPServer struct {
	db           *sql.DB
	analyzer     *analyzer.DatabaseAnalyzer
	knowledgeMgr *vectorstore.KnowledgeManager
	config       *config.Config
}

// NewMCPServer 創建 MCP 服務器
func NewMCPServer(db *sql.DB) *MCPServer {
	// 載入配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Printf("Warning: failed to load config: %v, using defaults", err)
		cfg = &config.Config{} // 使用默認配置
	}

	// 創建知識管理器
	var knowledgeMgr *vectorstore.KnowledgeManager
	if cfg.VectorStore.Enabled {
		knowledgeMgr, err = vectorstore.NewKnowledgeManager(cfg)
		if err != nil {
			log.Printf("Warning: failed to create knowledge manager: %v", err)
		}
	}

	return &MCPServer{
		db:           db,
		analyzer:     analyzer.NewDatabaseAnalyzer(db),
		knowledgeMgr: knowledgeMgr,
		config:       cfg,
	}
}

// Start 啟動 MCP 服務器
func (s *MCPServer) Start() error {
	log.Println("Starting MCP Server...")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 處理請求
		response, err := s.handleRequest(line)
		if err != nil {
			log.Printf("Error handling request: %v", err)
			continue
		}

		// 發送回應
		fmt.Println(response)
	}

	return scanner.Err()
}

// HandleRequest 處理請求（公共方法，用於測試）
func (s *MCPServer) HandleRequest(request string) (string, error) {
	return s.handleRequest(request)
}

// handleRequest 處理請求
func (s *MCPServer) handleRequest(request string) (string, error) {
	var req map[string]interface{}
	if err := json.Unmarshal([]byte(request), &req); err != nil {
		return "", fmt.Errorf("failed to parse request: %v", err)
	}

	method, ok := req["method"].(string)
	if !ok {
		return s.createErrorResponse(req, -32600, "Invalid Request")
	}

	switch method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return s.createErrorResponse(req, -32601, "Method not found")
	}
}

// handleInitialize 處理初始化請求
func (s *MCPServer) handleInitialize(req map[string]interface{}) (string, error) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req["id"],
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "aika-dba-mcp",
				"version": "1.0.0",
			},
		},
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// handleToolsList 處理工具列表請求
func (s *MCPServer) handleToolsList(req map[string]interface{}) (string, error) {
	tools := []map[string]interface{}{
		{
			"name":        "get_table_info",
			"description": "獲取特定資料表的所有資訊，包括 schema、constraints、indexes 和樣本數據",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "資料表名稱",
					},
				},
				"required": []string{"table_name"},
			},
		},
		{
			"name":        "execute_query",
			"description": "執行自定義 SQL 查詢並返回結果",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "要執行的 SQL 查詢",
					},
					"max_rows": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回行數，預設 100",
						"default":     100,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "get_more_samples",
			"description": "獲取特定資料表的更多樣本數據",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "資料表名稱",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "樣本數量，預設 50",
						"default":     50,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "偏移量，預設 0",
						"default":     0,
					},
				},
				"required": []string{"table_name"},
			},
		},
		{
			"name":        "get_database_schema_analysis",
			"description": "獲取 Phase 1 的資料庫架構分析結果，包括統計信息和樣本報告",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "查詢關鍵字，用於檢索相關的架構分析結果",
						"default":     "",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回結果數，預設 10",
						"default":     10,
					},
				},
			},
		},
		{
			"name":        "get_business_logic_analysis",
			"description": "獲取 Phase 2 的業務邏輯分析結果，包括 AI 洞察和建議",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "查詢關鍵字，用於檢索相關的業務邏輯分析",
						"default":     "",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回結果數，預設 10",
						"default":     10,
					},
				},
			},
		},
		{
			"name":        "get_comprehensive_business_overview",
			"description": "獲取 Phase 3 的綜合業務概覽，包括數據預處理和轉換計劃",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "查詢關鍵字，用於檢索相關的業務概覽",
						"default":     "",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回結果數，預設 10",
						"default":     10,
					},
				},
			},
		},
		{
			"name":        "get_dimensional_analysis",
			"description": "獲取 Phase 4 的維度分析結果，包括星形架構設計和 ETL 計劃",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "查詢關鍵字，用於檢索相關的維度分析",
						"default":     "",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回結果數，預設 10",
						"default":     10,
					},
				},
			},
		},
		{
			"name":        "get_knowledge_stats",
			"description": "獲取知識庫統計信息，包括各 phase 的知識塊數量",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req["id"],
		"result": map[string]interface{}{
			"tools": tools,
		},
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// handleToolsCall 處理工具調用請求
func (s *MCPServer) handleToolsCall(req map[string]interface{}) (string, error) {
	params, ok := req["params"].(map[string]interface{})
	if !ok {
		return s.createErrorResponse(req, -32602, "Invalid params")
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.createErrorResponse(req, -32602, "Invalid tool name")
	}

	toolArgs, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return s.createErrorResponse(req, -32602, "Invalid tool arguments")
	}

	var result interface{}
	var err error

	switch toolName {
	case "get_table_info":
		result, err = s.getTableInfo(toolArgs)
	case "execute_query":
		result, err = s.executeQuery(toolArgs)
	case "get_more_samples":
		result, err = s.getMoreSamples(toolArgs)
	case "get_database_schema_analysis":
		result, err = s.getDatabaseSchemaAnalysis(toolArgs)
	case "get_business_logic_analysis":
		result, err = s.getBusinessLogicAnalysis(toolArgs)
	case "get_comprehensive_business_overview":
		result, err = s.getComprehensiveBusinessOverview(toolArgs)
	case "get_dimensional_analysis":
		result, err = s.getDimensionalAnalysis(toolArgs)
	case "get_knowledge_stats":
		result, err = s.getKnowledgeStats(toolArgs)
	default:
		return s.createErrorResponse(req, -32601, "Tool not found")
	}

	if err != nil {
		return s.createErrorResponse(req, -32000, err.Error())
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req["id"],
		"result":  result,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// getTableInfo 獲取資料表資訊
func (s *MCPServer) getTableInfo(args map[string]interface{}) (interface{}, error) {
	tableName, ok := args["table_name"].(string)
	if !ok {
		return nil, fmt.Errorf("table_name is required")
	}

	log.Printf("Getting info for table: %s", tableName)

	// 獲取 schema
	schema, err := s.analyzer.GetTableSchema(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %v", err)
	}

	// 獲取 constraints
	constraints, err := s.analyzer.GetTableConstraints(tableName)
	if err != nil {
		log.Printf("Warning: failed to get constraints: %v", err)
		constraints = map[string]interface{}{}
	}

	// 獲取 indexes
	indexes, err := s.analyzer.GetTableIndexes(tableName)
	if err != nil {
		log.Printf("Warning: failed to get indexes: %v", err)
		indexes = []map[string]interface{}{}
	}

	// 獲取樣本數據
	samples, err := s.analyzer.GetTableSamples(tableName, 10) // 預設 10 個樣本
	if err != nil {
		log.Printf("Warning: failed to get samples: %v", err)
		samples = []map[string]interface{}{}
	}

	// 獲取統計信息
	stats, err := s.analyzer.GetTableStats(tableName)
	if err != nil {
		log.Printf("Warning: failed to get stats: %v", err)
		stats = map[string]interface{}{}
	}

	return map[string]interface{}{
		"table_name":  tableName,
		"schema":      schema,
		"constraints": constraints,
		"indexes":     indexes,
		"samples":     samples,
		"stats":       stats,
	}, nil
}

// executeQuery 執行自定義查詢
func (s *MCPServer) executeQuery(args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query is required")
	}

	maxRows := 100 // 預設最大行數
	if mr, ok := args["max_rows"]; ok {
		if maxRowsFloat, ok := mr.(float64); ok {
			maxRows = int(maxRowsFloat)
		}
	}

	log.Printf("Executing query: %s (max_rows: %d)", query, maxRows)

	// 檢查是否為 SELECT 查詢（為了安全）
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	// 獲取欄位名稱
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	// 讀取數據
	var results []map[string]interface{}
	count := 0
	for rows.Next() && count < maxRows {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val != nil {
				switch v := val.(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = v
				}
			} else {
				row[col] = nil
			}
		}

		results = append(results, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %v", err)
	}

	return map[string]interface{}{
		"query":     query,
		"columns":   columns,
		"rows":      results,
		"row_count": len(results),
		"truncated": count >= maxRows,
	}, nil
}

// getMoreSamples 獲取更多樣本數據
func (s *MCPServer) getMoreSamples(args map[string]interface{}) (interface{}, error) {
	tableName, ok := args["table_name"].(string)
	if !ok {
		return nil, fmt.Errorf("table_name is required")
	}

	limit := 50 // 預設 50 個樣本
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	offset := 0 // 預設偏移 0
	if o, ok := args["offset"]; ok {
		if offsetFloat, ok := o.(float64); ok {
			offset = int(offsetFloat)
		}
	}

	log.Printf("Getting more samples for table: %s (limit: %d, offset: %d)", tableName, limit, offset)

	// 使用現有的 GetTableSamples 方法，然後進行分頁
	allSamples, err := s.analyzer.GetTableSamples(tableName, limit+offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get samples: %v", err)
	}

	// 進行分頁
	var samples []map[string]interface{}
	start := offset
	end := offset + limit
	if start < len(allSamples) {
		if end > len(allSamples) {
			end = len(allSamples)
		}
		samples = allSamples[start:end]
	}

	return map[string]interface{}{
		"table_name": tableName,
		"samples":    samples,
		"limit":      limit,
		"offset":     offset,
		"count":      len(samples),
	}, nil
}

// createErrorResponse 創建錯誤回應
func (s *MCPServer) createErrorResponse(req map[string]interface{}, code int, message string) (string, error) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req["id"],
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// getDatabaseSchemaAnalysis 獲取 Phase 1 資料庫架構分析
func (s *MCPServer) getDatabaseSchemaAnalysis(args map[string]interface{}) (interface{}, error) {
	if s.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	limit := 10
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	log.Printf("Getting database schema analysis (query: %s, limit: %d)", query, limit)

	results, err := s.knowledgeMgr.RetrievePhaseKnowledge("phase1", query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve phase1 knowledge: %v", err)
	}

	return map[string]interface{}{
		"phase":       "phase1",
		"description": "Database statistical analysis with table schemas, constraints, and sample data",
		"query":       query,
		"results":     results,
		"count":       len(results),
	}, nil
}

// getBusinessLogicAnalysis 獲取 Phase 2 業務邏輯分析
func (s *MCPServer) getBusinessLogicAnalysis(args map[string]interface{}) (interface{}, error) {
	if s.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	limit := 10
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	log.Printf("Getting business logic analysis (query: %s, limit: %d)", query, limit)

	results, err := s.knowledgeMgr.RetrievePhaseKnowledge("phase2", query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve phase2 knowledge: %v", err)
	}

	return map[string]interface{}{
		"phase":       "phase2",
		"description": "AI-powered business logic analysis with LLM insights and recommendations",
		"query":       query,
		"results":     results,
		"count":       len(results),
	}, nil
}

// getComprehensiveBusinessOverview 獲取 Phase 3 綜合業務概覽
func (s *MCPServer) getComprehensiveBusinessOverview(args map[string]interface{}) (interface{}, error) {
	if s.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	limit := 10
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	log.Printf("Getting comprehensive business overview (query: %s, limit: %d)", query, limit)

	results, err := s.knowledgeMgr.RetrievePhaseKnowledge("phase3", query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve phase3 knowledge: %v", err)
	}

	return map[string]interface{}{
		"phase":       "phase3",
		"description": "Data preprocessing and transformation preparation",
		"query":       query,
		"results":     results,
		"count":       len(results),
	}, nil
}

// getDimensionalAnalysis 獲取 Phase 4 維度分析
func (s *MCPServer) getDimensionalAnalysis(args map[string]interface{}) (interface{}, error) {
	if s.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	limit := 10
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	log.Printf("Getting dimensional analysis (query: %s, limit: %d)", query, limit)

	results, err := s.knowledgeMgr.RetrievePhaseKnowledge("phase4", query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve phase4 knowledge: %v", err)
	}

	return map[string]interface{}{
		"phase":       "phase4",
		"description": "Dimensional modeling with star schema design and ETL planning",
		"query":       query,
		"results":     results,
		"count":       len(results),
	}, nil
}

// getKnowledgeStats 獲取知識統計信息
func (s *MCPServer) getKnowledgeStats(args map[string]interface{}) (interface{}, error) {
	if s.knowledgeMgr == nil {
		return nil, fmt.Errorf("knowledge manager not available")
	}

	log.Printf("Getting knowledge statistics")

	stats, err := s.knowledgeMgr.GetKnowledgeStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge stats: %v", err)
	}

	return map[string]interface{}{
		"knowledge_stats":      stats,
		"vector_store_enabled": s.config.VectorStore.Enabled,
		"database_path":        s.config.VectorStore.DatabasePath,
	}, nil
}
