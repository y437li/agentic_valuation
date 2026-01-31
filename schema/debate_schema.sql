-- Debate System Schema

-- 1. Debates Table
CREATE TABLE debates (
    id UUID PRIMARY KEY,
    company VARCHAR(255) NOT NULL,
    fiscal_year VARCHAR(20) NOT NULL,
    status VARCHAR(50) NOT NULL, -- 'idle', 'running', 'completed', 'failed'
    
    -- Final Report stored as JSONB
    report JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Debate Messages Table (The Transcript)
CREATE TABLE debate_messages (
    id SERIAL PRIMARY KEY,
    debate_id UUID REFERENCES debates(id) ON DELETE CASCADE,
    round_index INTEGER NOT NULL,
    agent_role VARCHAR(50) NOT NULL,
    agent_name VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    
    -- Store references/citations as JSONB array
    references JSONB,
    
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster history retrieval
CREATE INDEX idx_debate_messages_debate_id ON debate_messages(debate_id);
