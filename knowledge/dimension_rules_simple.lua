-- Simple test dimension rules
-- Generated on: 2025-11-05
-- Database: ecommerce_db

-- Helper functions
function contains(table, element)
    for _, value in pairs(table) do
        if value == element then
            return true
        end
    end
    return false
end

function has_any_field(table_meta, fields)
    if not table_meta.columns then
        return false
    end
    for _, col in pairs(table_meta.columns) do
        if col.name then
            for _, field in ipairs(fields) do
                if col.name == field then
                    return true
                end
            end
        end
    end
    return false
end

-- Dimension detection function
function detect_dimensions(table_name, table_meta)
    local dimensions = {}

    -- Simple rule-based dimension detection
    if table_name == "customers" then
        table.insert(dimensions, {
            name = "dim_customer",
            type = "people",
            description = "Customer dimension table",
            source_table = table_name,
            key_fields = {"id", "email"},
            attributes = {"name", "phone", "date_of_birth", "gender"},
            business_use = "Used for customer analysis"
        })
    elseif table_name == "products" then
        table.insert(dimensions, {
            name = "dim_product",
            type = "product",
            description = "Product dimension table",
            source_table = table_name,
            key_fields = {"id", "sku"},
            attributes = {"name", "description", "price", "category_id"},
            business_use = "Used for product analysis"
        })
    end

    return dimensions
end

-- Fact table detection function
function detect_fact_tables(table_name, table_meta)
    local fact_tables = {}

    -- Simple rule-based fact table detection
    if has_any_field(table_meta, {"order_id", "quantity", "amount", "total"}) then
        table.insert(fact_tables, {
            name = "fact_sales",
            description = "Sales fact table",
            source_table = table_name,
            measures = {"quantity", "amount", "total", "discount"},
            dimensions = {"dim_date", "dim_customer", "dim_product", "dim_location"}
        })
    end

    return fact_tables
end

return {
    detect_dimensions = detect_dimensions,
    detect_fact_tables = detect_fact_tables
}