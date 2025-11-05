package analyzer

import (
	"testing"
	"github.com/masato25/aika-dba/internal/schema"
)

func TestDatabaseAnalyzer_quoteIdentifier(t *testing.T) {
	tests := []struct {
		dbType     string
		identifier string
		expected   string
	}{
		{"mysql", "table_name", "`table_name`"},
		{"postgres", "table_name", `"table_name"`},
		{"sqlite", "table_name", "table_name"},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			analyzer := &DatabaseAnalyzer{dbType: tt.dbType}
			result := analyzer.quoteIdentifier(tt.identifier)
			if result != tt.expected {
				t.Errorf("quoteIdentifier() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDatabaseAnalyzer_isNumericType(t *testing.T) {
	analyzer := &DatabaseAnalyzer{}

	tests := []struct {
		columnType string
		expected   bool
	}{
		{"int", true},
		{"integer", true},
		{"bigint", true},
		{"decimal", true},
		{"numeric", true},
		{"float", true},
		{"double", true},
		{"varchar", false},
		{"text", false},
		{"timestamp", false},
	}

	for _, tt := range tests {
		t.Run(tt.columnType, func(t *testing.T) {
			result := analyzer.isNumericType(tt.columnType)
			if result != tt.expected {
				t.Errorf("isNumericType(%s) = %v, want %v", tt.columnType, result, tt.expected)
			}
		})
	}
}

func TestDatabaseAnalyzer_generateTableSummary(t *testing.T) {
	analyzer := &DatabaseAnalyzer{}

	// 模擬表格和欄位分析
	table := schema.Table{
		Name: "test_table",
		Columns: []schema.Column{
			{Name: "id", IsPrimaryKey: true, IsForeignKey: false},
			{Name: "name", IsPrimaryKey: false, IsForeignKey: false},
			{Name: "ref_id", IsPrimaryKey: false, IsForeignKey: true},
		},
	}

	columnAnalyses := []ColumnAnalysis{
		{ColumnName: "id", TotalCount: 100, NotNullCount: 100, NullCount: 0},
		{ColumnName: "name", TotalCount: 100, NotNullCount: 80, NullCount: 20},
		{ColumnName: "ref_id", TotalCount: 100, NotNullCount: 95, NullCount: 5},
	}

	summary := analyzer.generateTableSummary(table, columnAnalyses, 100)

	if summary.TotalColumns != 3 {
		t.Errorf("TotalColumns = %v, want %v", summary.TotalColumns, 3)
	}

	if summary.PrimaryKeys != 1 {
		t.Errorf("PrimaryKeys = %v, want %v", summary.PrimaryKeys, 1)
	}

	if summary.ForeignKeys != 1 {
		t.Errorf("ForeignKeys = %v, want %v", summary.ForeignKeys, 1)
	}

	expectedCompleteness := (100.0 + 80.0 + 95.0) / 300.0 // 平均完整性
	if summary.DataCompleteness < expectedCompleteness-0.01 || summary.DataCompleteness > expectedCompleteness+0.01 {
		t.Errorf("DataCompleteness = %v, want ~%v", summary.DataCompleteness, expectedCompleteness)
	}
}