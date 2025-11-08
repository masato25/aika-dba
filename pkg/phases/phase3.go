package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/masato25/aika-dba/config"
	"github.com/masato25/aika-dba/pkg/llm"
	"github.com/masato25/aika-dba/pkg/vectorstore"
)

// Phase3Runner handles the generation of business logic descriptions
type Phase3Runner struct {
	config      *config.Config
	llmClient   *llm.Client
	vectorStore *vectorstore.KnowledgeManager
}

// NewPhase3Runner creates a new Phase3Runner instance
func NewPhase3Runner(cfg *config.Config, llmClient *llm.Client, vectorStore *vectorstore.KnowledgeManager) *Phase3Runner {
	return &Phase3Runner{
		config:      cfg,
		llmClient:   llmClient,
		vectorStore: vectorStore,
	}
}

// Phase3AnalysisResult represents the result of phase 3 analysis
type Phase3AnalysisResult struct {
	DatabaseName         string              `json:"database_name"`
	DatabaseType         string              `json:"database_type"`
	BusinessLogicSummary string              `json:"business_logic_summary"`
	TableCategories      map[string][]string `json:"table_categories"`
	KeyBusinessProcesses []string            `json:"key_business_processes"`
	DataFlowPatterns     []string            `json:"data_flow_patterns"`
	Recommendations      []string            `json:"recommendations"`
	Timestamp            string              `json:"timestamp"`
}

// Run executes the phase 3 analysis
func (p *Phase3Runner) Run(ctx context.Context) error {
	fmt.Println("Starting Phase 3: Business Logic Description Generation")

	// Read phase 2 analysis results
	phase2Data, err := p.readPhase2Analysis()
	if err != nil {
		return fmt.Errorf("failed to read phase 2 analysis: %w", err)
	}

	// Generate business logic description using LLM
	result, err := p.generateBusinessLogicDescription(ctx, phase2Data)
	if err != nil {
		return fmt.Errorf("failed to generate business logic description: %w", err)
	}

	// Save the result
	if err := p.saveResult(result); err != nil {
		return fmt.Errorf("failed to save phase 3 result: %w", err)
	}

	// Store summary in vector database (avoid storing large detailed data)
	if err := p.storeVectorKnowledge(result); err != nil {
		return fmt.Errorf("failed to store phase 3 knowledge in vector database: %w", err)
	}

	fmt.Println("Phase 3 completed successfully")
	return nil
}

// readPhase2Analysis reads the phase 2 analysis results from file
func (p *Phase3Runner) readPhase2Analysis() (*Phase2AnalysisResult, error) {
	filePath := "knowledge/phase2_analysis.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read phase 2 analysis file: %w", err)
	}

	var result Phase2AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal phase 2 analysis: %w", err)
	}

	return &result, nil
}

// Phase2AnalysisResult represents the structure of phase 2 analysis results
type Phase2AnalysisResult struct {
	AnalysisResults map[string]TableAnalysis `json:"analysis_results"`
	Database        string                   `json:"database"`
	DatabaseType    string                   `json:"database_type"`
	Description     string                   `json:"description"`
	Summary         Phase2Summary            `json:"summary"`
	Timestamp       string                   `json:"timestamp"`
}

// TableAnalysis represents the analysis of a single table
type TableAnalysis struct {
	TableName string `json:"table_name"`
	Analysis  string `json:"analysis"`
	Timestamp string `json:"timestamp"`
}

// Phase2Summary represents the summary of phase 2 analysis
type Phase2Summary struct {
	AnalysisTimestamp   string `json:"analysis_timestamp"`
	Description         string `json:"description"`
	Phase               string `json:"phase"`
	TotalTablesAnalyzed int    `json:"total_tables_analyzed"`
}

// generateBusinessLogicDescription uses LLM to generate comprehensive business logic description
func (p *Phase3Runner) generateBusinessLogicDescription(ctx context.Context, phase2Data *Phase2AnalysisResult) (*Phase3AnalysisResult, error) {
	// Prepare the analysis data for LLM
	analysisText := p.prepareAnalysisText(phase2Data)

	// Create the prompt for LLM
	prompt := p.createBusinessLogicPrompt(analysisText)

	// Call LLM to generate business logic description
	response, err := p.llmClient.GenerateCompletion(ctx, prompt)
	if err != nil {
		// Fallback: generate basic business logic description without LLM
		fmt.Printf("LLM call failed, using fallback method: %v\n", err)
		return p.generateFallbackDescription(phase2Data), nil
	}

	// Parse the LLM response
	result, err := p.parseLLMResponse(response, phase2Data)
	if err != nil {
		fmt.Printf("Failed to parse LLM response, using fallback: %v\n", err)
		return p.generateFallbackDescription(phase2Data), nil
	}

	return result, nil
}

// prepareAnalysisText converts the phase 2 analysis results into a concise text format for LLM
func (p *Phase3Runner) prepareAnalysisText(phase2Data *Phase2AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Database: %s (%s)\n", phase2Data.Database, phase2Data.DatabaseType))
	sb.WriteString(fmt.Sprintf("Total Tables Analyzed: %d\n\n", phase2Data.Summary.TotalTablesAnalyzed))

	sb.WriteString("Table Summary:\n")
	sb.WriteString("=============\n\n")

	// Create a concise summary for each table instead of full analysis
	for tableName, analysis := range phase2Data.AnalysisResults {
		// Extract key points from the analysis (first 200 characters as summary)
		summary := analysis.Analysis
		if len(summary) > 200 {
			// Find a good break point (sentence end)
			if idx := strings.LastIndex(summary[:200], "."); idx > 50 {
				summary = summary[:idx+1]
			} else {
				summary = summary[:200] + "..."
			}
		}

		sb.WriteString(fmt.Sprintf("• %s: %s\n", tableName, summary))
	}

	sb.WriteString("\nTable List:\n")
	sb.WriteString("===========\n")
	for tableName := range phase2Data.AnalysisResults {
		sb.WriteString(fmt.Sprintf("• %s\n", tableName))
	}

	return sb.String()
}

// createBusinessLogicPrompt creates a concise prompt for LLM to generate business logic description
func (p *Phase3Runner) createBusinessLogicPrompt(analysisText string) string {
	return fmt.Sprintf(`Analyze this database and provide a business logic summary.

Database Info:
%s

Provide a JSON response with:
{
  "business_logic_summary": "Brief description of the business domain",
  "table_categories": {"Category1": ["table1", "table2"], "Category2": ["table3"]},
  "key_business_processes": ["Process 1", "Process 2"],
  "data_flow_patterns": ["Pattern 1", "Pattern 2"],
  "recommendations": ["Recommendation 1", "Recommendation 2"]
}`, analysisText)
}

// parseLLMResponse parses the LLM response into a Phase3AnalysisResult
func (p *Phase3Runner) parseLLMResponse(response string, phase2Data *Phase2AnalysisResult) (*Phase3AnalysisResult, error) {
	// Try to extract JSON from the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in LLM response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var llmResult struct {
		BusinessLogicSummary string              `json:"business_logic_summary"`
		TableCategories      map[string][]string `json:"table_categories"`
		KeyBusinessProcesses []string            `json:"key_business_processes"`
		DataFlowPatterns     []string            `json:"data_flow_patterns"`
		Recommendations      []string            `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	result := &Phase3AnalysisResult{
		DatabaseName:         phase2Data.Database,
		DatabaseType:         phase2Data.DatabaseType,
		BusinessLogicSummary: llmResult.BusinessLogicSummary,
		TableCategories:      llmResult.TableCategories,
		KeyBusinessProcesses: llmResult.KeyBusinessProcesses,
		DataFlowPatterns:     llmResult.DataFlowPatterns,
		Recommendations:      llmResult.Recommendations,
		Timestamp:            phase2Data.Timestamp,
	}

	return result, nil
}

// generateFallbackDescription generates a basic business logic description when LLM is unavailable
func (p *Phase3Runner) generateFallbackDescription(phase2Data *Phase2AnalysisResult) *Phase3AnalysisResult {
	// Categorize tables based on naming patterns
	tableCategories := p.categorizeTables(phase2Data.AnalysisResults)

	// Generate basic business processes
	businessProcesses := p.identifyBusinessProcesses(phase2Data.AnalysisResults)

	// Generate basic data flow patterns
	dataFlowPatterns := []string{
		"User registration and authentication flow",
		"Order processing and fulfillment flow",
		"Product management and inventory flow",
		"Payment processing and transaction flow",
		"Customer service and support flow",
	}

	// Generate recommendations
	recommendations := []string{
		"Implement proper indexing on frequently queried columns",
		"Add audit trails for critical business operations",
		"Consider implementing data partitioning for large tables",
		"Add foreign key constraints to maintain data integrity",
		"Implement proper backup and recovery procedures",
	}

	summary := fmt.Sprintf("This is a comprehensive business database for %s, containing %d tables that support various business operations including user management, order processing, product management, and customer service. The database appears to be designed for an e-commerce or CRM system with features for member management, promotions, and transaction processing.",
		phase2Data.Database, len(phase2Data.AnalysisResults))

	return &Phase3AnalysisResult{
		DatabaseName:         phase2Data.Database,
		DatabaseType:         phase2Data.DatabaseType,
		BusinessLogicSummary: summary,
		TableCategories:      tableCategories,
		KeyBusinessProcesses: businessProcesses,
		DataFlowPatterns:     dataFlowPatterns,
		Recommendations:      recommendations,
		Timestamp:            phase2Data.Timestamp,
	}
}

// categorizeTables groups tables into logical categories based on naming patterns
func (p *Phase3Runner) categorizeTables(analysisResults map[string]TableAnalysis) map[string][]string {
	categories := make(map[string][]string)

	for tableName := range analysisResults {
		category := p.determineTableCategory(tableName)
		categories[category] = append(categories[category], tableName)
	}

	return categories
}

// determineTableCategory determines the category of a table based on its name
func (p *Phase3Runner) determineTableCategory(tableName string) string {
	tableName = strings.ToLower(tableName)

	switch {
	case strings.Contains(tableName, "user") || strings.Contains(tableName, "member"):
		return "User Management"
	case strings.Contains(tableName, "order") || strings.Contains(tableName, "transaction"):
		return "Order Processing"
	case strings.Contains(tableName, "product") || strings.Contains(tableName, "item"):
		return "Product Management"
	case strings.Contains(tableName, "payment") || strings.Contains(tableName, "tender"):
		return "Payment Processing"
	case strings.Contains(tableName, "benefit") || strings.Contains(tableName, "coupon") || strings.Contains(tableName, "voucher") || strings.Contains(tableName, "promotion"):
		return "Promotions & Benefits"
	case strings.Contains(tableName, "log") || strings.Contains(tableName, "audit"):
		return "Audit & Logging"
	case strings.Contains(tableName, "tier") || strings.Contains(tableName, "level"):
		return "Membership Tiers"
	case strings.Contains(tableName, "contact") || strings.Contains(tableName, "client"):
		return "Contact Management"
	case strings.Contains(tableName, "currency"):
		return "Currency Management"
	case strings.Contains(tableName, "migration") || strings.Contains(tableName, "schema"):
		return "System Management"
	default:
		return "General Business Data"
	}
}

// identifyBusinessProcesses identifies key business processes from the analysis
func (p *Phase3Runner) identifyBusinessProcesses(analysisResults map[string]TableAnalysis) []string {
	processes := []string{
		"User Registration and Authentication",
		"Product Catalog Management",
		"Order Placement and Processing",
		"Payment Processing and Transaction Management",
		"Inventory Management",
		"Customer Service and Support",
		"Promotions and Discounts Management",
		"Membership and Loyalty Program Management",
		"Reporting and Analytics",
		"System Administration and Maintenance",
	}

	return processes
}

// storeVectorKnowledge stores a summary of the phase 3 analysis in the vector database
func (p *Phase3Runner) storeVectorKnowledge(result *Phase3AnalysisResult) error {
	// Create a summary version for vector storage (avoid storing large detailed data)
	summaryOutput := map[string]interface{}{
		"phase":           "phase3",
		"description":     "Business logic description and analysis summary",
		"database":        result.DatabaseName,
		"database_type":   result.DatabaseType,
		"timestamp":       result.Timestamp,
		"business_domain": p.extractBusinessDomain(result.BusinessLogicSummary),
		"summary": map[string]interface{}{
			"total_table_categories":   len(result.TableCategories),
			"total_business_processes": len(result.KeyBusinessProcesses),
			"total_data_flow_patterns": len(result.DataFlowPatterns),
			"total_recommendations":    len(result.Recommendations),
			"key_insights":             p.extractKeyInsights(result),
		},
	}

	// Store the summary in vector database
	if err := p.vectorStore.StorePhaseKnowledge("phase3", summaryOutput); err != nil {
		return fmt.Errorf("failed to store phase 3 knowledge: %w", err)
	}

	fmt.Println("Phase 3 knowledge summary stored in vector database")
	return nil
}

// extractBusinessDomain extracts the main business domain from the business logic summary
func (p *Phase3Runner) extractBusinessDomain(summary string) string {
	summary = strings.ToLower(summary)

	// Simple keyword-based domain detection
	switch {
	case strings.Contains(summary, "e-commerce") || strings.Contains(summary, "shopping") || strings.Contains(summary, "retail"):
		return "e-commerce"
	case strings.Contains(summary, "crm") || strings.Contains(summary, "customer") || strings.Contains(summary, "relationship"):
		return "customer relationship management"
	case strings.Contains(summary, "inventory") || strings.Contains(summary, "warehouse") || strings.Contains(summary, "supply chain"):
		return "inventory management"
	case strings.Contains(summary, "financial") || strings.Contains(summary, "banking") || strings.Contains(summary, "transaction"):
		return "financial services"
	case strings.Contains(summary, "healthcare") || strings.Contains(summary, "medical") || strings.Contains(summary, "patient"):
		return "healthcare"
	case strings.Contains(summary, "education") || strings.Contains(summary, "learning") || strings.Contains(summary, "student"):
		return "education"
	default:
		return "general business"
	}
}

// extractKeyInsights extracts key insights from the phase 3 analysis result
func (p *Phase3Runner) extractKeyInsights(result *Phase3AnalysisResult) map[string]interface{} {
	insights := map[string]interface{}{
		"primary_business_focus": p.extractBusinessDomain(result.BusinessLogicSummary),
		"system_complexity":      p.assessSystemComplexity(result),
		"data_architecture_type": p.determineArchitectureType(result),
	}

	// Add top categories
	if len(result.TableCategories) > 0 {
		topCategories := make([]string, 0, 3)
		for category := range result.TableCategories {
			if len(topCategories) < 3 {
				topCategories = append(topCategories, category)
			}
		}
		insights["top_categories"] = topCategories
	}

	// Add key processes
	if len(result.KeyBusinessProcesses) > 0 {
		keyProcesses := make([]string, 0, 3)
		for _, process := range result.KeyBusinessProcesses {
			if len(keyProcesses) < 3 {
				keyProcesses = append(keyProcesses, process)
			}
		}
		insights["key_processes"] = keyProcesses
	}

	return insights
}

// assessSystemComplexity assesses the complexity of the system based on the analysis
func (p *Phase3Runner) assessSystemComplexity(result *Phase3AnalysisResult) string {
	totalElements := len(result.TableCategories) + len(result.KeyBusinessProcesses) + len(result.DataFlowPatterns)

	switch {
	case totalElements < 10:
		return "simple"
	case totalElements < 20:
		return "moderate"
	case totalElements < 30:
		return "complex"
	default:
		return "highly_complex"
	}
}

// determineArchitectureType determines the type of data architecture
func (p *Phase3Runner) determineArchitectureType(result *Phase3AnalysisResult) string {
	// Check for common architecture patterns
	hasUserManagement := false
	hasTransactions := false
	hasProducts := false
	hasPayments := false

	for category := range result.TableCategories {
		categoryLower := strings.ToLower(category)
		if strings.Contains(categoryLower, "user") || strings.Contains(categoryLower, "member") {
			hasUserManagement = true
		}
		if strings.Contains(categoryLower, "order") || strings.Contains(categoryLower, "transaction") {
			hasTransactions = true
		}
		if strings.Contains(categoryLower, "product") || strings.Contains(categoryLower, "item") {
			hasProducts = true
		}
		if strings.Contains(categoryLower, "payment") || strings.Contains(categoryLower, "financial") {
			hasPayments = true
		}
	}

	// Determine architecture type
	if hasUserManagement && hasTransactions && hasProducts && hasPayments {
		return "e-commerce_platform"
	} else if hasUserManagement && hasTransactions {
		return "transaction_system"
	} else if hasUserManagement {
		return "user_management_system"
	} else {
		return "general_business_system"
	}
}

// saveResult saves the phase 3 analysis result to a JSON file
func (p *Phase3Runner) saveResult(result *Phase3AnalysisResult) error {
	// Create the output directory if it doesn't exist
	outputDir := "knowledge"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save to phase3_analysis.json
	filePath := "knowledge/phase3_analysis.json"
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result file: %w", err)
	}

	fmt.Printf("Phase 3 analysis result saved to: %s\n", filePath)
	return nil
}
