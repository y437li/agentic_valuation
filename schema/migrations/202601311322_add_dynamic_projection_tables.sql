-- Migration: Add Dynamic Schema for Projections
-- Version: 202601311322
-- Description: Creates projection_assumptions, common_size_baselines, and projected_financials tables
--              with JSONB columns for dynamic NodeDrivers, CustomItems, and SegmentGrowth storage.

-- ============================================================
-- Table: projection_assumptions
-- Stores complete ProjectionAssumptions with dynamic JSONB fields
-- ============================================================
CREATE TABLE IF NOT EXISTS projection_assumptions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    case_id UUID REFERENCES cases(id) ON DELETE CASCADE,
    period_id BIGINT REFERENCES financial_periods(id) ON DELETE SET NULL,
    company_id BIGINT REFERENCES companies(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL,
    
    -- Core Fixed Drivers (frequently queried)
    revenue_growth NUMERIC(10, 6),
    cogs_percent NUMERIC(10, 6),
    sga_percent NUMERIC(10, 6),
    rd_percent NUMERIC(10, 6),
    tax_rate NUMERIC(10, 6),
    capex_percent NUMERIC(10, 6),
    terminal_growth NUMERIC(10, 6),
    
    -- Dynamic JSONB Fields
    node_drivers JSONB DEFAULT '{}',      -- {"IS-OpCost: SBC": 0.05, "BS-CA: Prepaid": 0.02}
    segment_growth JSONB DEFAULT '{}',    -- {"Automotive": 0.08, "Ford Credit": 0.03}
    wacc_components JSONB DEFAULT '{}',   -- {"unlevered_beta": 1.1, "risk_free_rate": 0.04}
    working_capital JSONB DEFAULT '{}',   -- {"dso": 45, "dsi": 60, "dpo": 30}
    
    -- Metadata
    source TEXT DEFAULT 'calculated',     -- 'calculated', 'manual', 'agent'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- Table: common_size_baselines
-- Stores calc.CommonSizeDefaults with dynamic CustomItems
-- ============================================================
CREATE TABLE IF NOT EXISTS common_size_baselines (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    company_id BIGINT REFERENCES companies(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL,
    accession_number TEXT,
    
    -- Core Metrics
    cogs_percent NUMERIC(10, 6),
    sga_percent NUMERIC(10, 6),
    rd_percent NUMERIC(10, 6),
    effective_tax_rate NUMERIC(10, 6),
    depreciation_percent NUMERIC(10, 6),
    
    -- Dynamic Item-Level Drivers
    custom_items JSONB DEFAULT '{}',  -- {"selling_marketing_pct": 0.12, "impairment_pct": 0.001}
    
    -- Working Capital Percentages
    receivables_percent NUMERIC(10, 6),
    inventory_percent NUMERIC(10, 6),
    ap_percent NUMERIC(10, 6),
    deferred_rev_percent NUMERIC(10, 6),
    
    -- Metadata
    source_extraction_id UUID REFERENCES fsap_extractions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(company_id, fiscal_year)
);

-- ============================================================
-- Table: projected_financials
-- Caches projected financial statements as JSONB
-- ============================================================
CREATE TABLE IF NOT EXISTS projected_financials (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    case_id UUID REFERENCES cases(id) ON DELETE CASCADE,
    company_id BIGINT REFERENCES companies(id) ON DELETE CASCADE,
    target_year INT NOT NULL,
    base_year INT NOT NULL,
    
    -- Full Statements (JSONB for flexibility)
    income_statement JSONB NOT NULL,
    balance_sheet JSONB NOT NULL,
    cash_flow_statement JSONB NOT NULL,
    segments JSONB DEFAULT '[]',          -- Projected segments for SOTP
    
    -- Key Summary Metrics (for quick queries)
    projected_revenue NUMERIC(20, 2),
    projected_net_income NUMERIC(20, 2),
    projected_fcf NUMERIC(20, 2),
    
    -- Link to assumptions used
    assumptions_id UUID REFERENCES projection_assumptions(id) ON DELETE SET NULL,
    
    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(case_id, target_year)
);

-- ============================================================
-- Indexes for Performance
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_projection_assumptions_case ON projection_assumptions(case_id);
CREATE INDEX IF NOT EXISTS idx_projection_assumptions_company ON projection_assumptions(company_id);
CREATE INDEX IF NOT EXISTS idx_common_size_baselines_company ON common_size_baselines(company_id);
CREATE INDEX IF NOT EXISTS idx_projected_financials_case ON projected_financials(case_id);

-- GIN Indexes for JSONB queries
CREATE INDEX IF NOT EXISTS idx_projection_assumptions_node_drivers ON projection_assumptions USING GIN (node_drivers);
CREATE INDEX IF NOT EXISTS idx_common_size_baselines_custom_items ON common_size_baselines USING GIN (custom_items);

-- ============================================================
-- Updated_at Triggers
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_projection_assumptions_updated_at
    BEFORE UPDATE ON projection_assumptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_projected_financials_updated_at
    BEFORE UPDATE ON projected_financials
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
