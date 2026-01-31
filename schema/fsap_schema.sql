-- Database Schema for Agentic Valuation Platform (FSAP Edition) - Granular Version

-- 1. Companies
CREATE TABLE companies (
    id SERIAL PRIMARY KEY,
    ticker VARCHAR(10) UNIQUE NOT NULL,
    company_name VARCHAR(255) NOT NULL,
    sector VARCHAR(100),
    industry VARCHAR(100),
    currency VARCHAR(10) DEFAULT 'USD',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Financial Periods
CREATE TABLE financial_periods (
    id SERIAL PRIMARY KEY,
    company_id INTEGER REFERENCES companies(id),
    period_type VARCHAR(2) NOT NULL, -- 'FY', 'Q1', etc.
    fiscal_year INTEGER NOT NULL,
    end_date DATE NOT NULL,
    is_audit_complete BOOLEAN DEFAULT FALSE,
    UNIQUE(company_id, fiscal_year, period_type)
);

-- 3. Granular FSAP Data
-- Reflecting the exact structure from the Excel template
CREATE TABLE fsap_data (
    id SERIAL PRIMARY KEY,
    period_id INTEGER REFERENCES financial_periods(id),
    snapshot_id INTEGER, -- Link to project_snapshots (added via Alter later or defined now)
    
    -- Balance Sheet (Assets)
    cash_equiv NUMERIC(20, 2),
    st_invest NUMERIC(20, 2),
    ar_net NUMERIC(20, 2),
    inventories NUMERIC(20, 2),
    finance_loans_st NUMERIC(20, 2),
    finance_other_ca NUMERIC(20, 2),
    other_ca_1 NUMERIC(20, 2),
    other_ca_2 NUMERIC(20, 2),
    total_current_assets NUMERIC(20, 2),
    lt_invest NUMERIC(20, 2),
    ppe_cost NUMERIC(20, 2),
    accum_depreciation NUMERIC(20, 2),
    ppe_net NUMERIC(20, 2),
    intangibles NUMERIC(20, 2),
    deferred_tax_assets_lt NUMERIC(20, 2),
    total_assets NUMERIC(20, 2),
    
    -- Balance Sheet (Liabilities & Equity)
    ap NUMERIC(20, 2),
    accrued_liab NUMERIC(20, 2),
    notes_payable_st_debt NUMERIC(20, 2),
    curr_portion_lt_debt NUMERIC(20, 2),
    total_current_liabilities NUMERIC(20, 2),
    lt_debt NUMERIC(20, 2),
    deferred_tax_liab_lt NUMERIC(20, 2),
    total_liabilities NUMERIC(20, 2),
    common_stock NUMERIC(20, 2),
    retained_earnings NUMERIC(20, 2),
    total_equity NUMERIC(20, 2),
    total_liab_equity NUMERIC(20, 2),
    
    -- Income Statement
    revenues NUMERIC(20, 2),
    cogs NUMERIC(20, 2),
    gross_profit NUMERIC(20, 2),
    sga NUMERIC(20, 2),
    advertising_exp NUMERIC(20, 2),
    rd_exp NUMERIC(20, 2),
    op_income NUMERIC(20, 2),
    interest_exp NUMERIC(20, 2),
    pretax_income NUMERIC(20, 2),
    income_tax_exp NUMERIC(20, 2),
    net_income NUMERIC(20, 2),

    -- Audit & Context
    audit_trail JSONB, -- { "field": { "note": "...", "page": 12, "logic": "..." } }
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Snapshots for Version Control
CREATE TABLE project_snapshots (
    id SERIAL PRIMARY KEY,
    company_id INTEGER REFERENCES companies(id),
    version_label VARCHAR(100),
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. User Training Logs / RLHF Feedback Loop
-- Captures corrections to mappings, assumptions, and values to align Agent behavior
CREATE TABLE user_training_logs (
    id SERIAL PRIMARY KEY,
    user_id UUID, -- For multi-user preference alignment
    period_id INTEGER REFERENCES financial_periods(id),
    action_type VARCHAR(50), -- 'RECLASSIFY', 'ADJUST_VALUE', 'OVERRIDE_ASSUMPTION', 'REJECT_EVIDENCE'
    
    -- Entity details
    item_key VARCHAR(100), -- The standardized variable name (e.g., 'rev_growth_f')
    original_value JSONB,  -- Original agent output (can be numeric or complex object)
    revised_value JSONB,   -- User's correction
    
    -- Reasoning & Alignment Context
    agent_reasoning TEXT,
    user_reason TEXT,
    
    -- High-fidelity context for RLHF
    -- Stores source PDF snippet, prompt version, and model ID at time of feedback
    context_snapshot JSONB, 
    
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
