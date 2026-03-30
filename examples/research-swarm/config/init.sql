-- Spans database initialization for LiteLLM and agent tracing
-- Created automatically when postgres container starts

-- spans_trace table: Capture structured trace events for each agent
CREATE TABLE IF NOT EXISTS spans_trace (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    span_id VARCHAR(36) NOT NULL,
    parent_span_id VARCHAR(36),
    agent_role VARCHAR(50) NOT NULL,  -- researcher, writer, editor
    agent_tone VARCHAR(50),
    operation VARCHAR(100) NOT NULL,  -- /research, /write, /edit
    status VARCHAR(20) NOT NULL,  -- pending, running, completed, failed
    input_tokens INT,
    output_tokens INT,
    cost_usd DECIMAL(10, 6),
    virtual_key VARCHAR(100),
    start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP,
    duration_ms INT,
    error_message TEXT,
    model_used VARCHAR(50),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indices for fast queries
CREATE INDEX spans_trace_trace_id ON spans_trace(trace_id);
CREATE INDEX spans_trace_agent_role ON spans_trace(agent_role);
CREATE INDEX spans_trace_status ON spans_trace(status);
CREATE INDEX spans_trace_created_at ON spans_trace(created_at DESC);

-- litellm_spend table: Aggregate spend per virtual key per day
CREATE TABLE IF NOT EXISTS litellm_spend (
    id SERIAL PRIMARY KEY,
    virtual_key VARCHAR(100) NOT NULL,
    user_id VARCHAR(100),
    team_id VARCHAR(100),
    total_cost_usd DECIMAL(10, 6) NOT NULL DEFAULT 0,
    total_input_tokens INT,
    total_output_tokens INT,
    request_count INT DEFAULT 0,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(virtual_key, date)
);

CREATE INDEX litellm_spend_virtual_key ON litellm_spend(virtual_key);
CREATE INDEX litellm_spend_date ON litellm_spend(date DESC);

-- artifacts table: Track MinIO artifact references
CREATE TABLE IF NOT EXISTS artifacts (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    artifact_type VARCHAR(50) NOT NULL,  -- research_outline, draft, final_output, changelog
    minio_path VARCHAR(255) NOT NULL UNIQUE,
    content_hash VARCHAR(64),
    size_bytes INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (trace_id) REFERENCES spans_trace(trace_id)
);

CREATE INDEX artifacts_trace_id ON artifacts(trace_id);
CREATE INDEX artifacts_artifact_type ON artifacts(artifact_type);

-- Seed data: Initialize spend tracking for agents
INSERT INTO litellm_spend (virtual_key, user_id, team_id, date)
VALUES
    ('sk-researcher-virtual', 'researcher-agent', 'agentic-demo', CURRENT_DATE),
    ('sk-writer-virtual', 'writer-agent', 'agentic-demo', CURRENT_DATE),
    ('sk-editor-virtual', 'editor-agent', 'agentic-demo', CURRENT_DATE)
ON CONFLICT (virtual_key, date) DO NOTHING;

-- Create schema comments for IDE autocomplete
COMMENT ON TABLE spans_trace IS 'Structured trace events for each agent operation in the research pipeline';
COMMENT ON TABLE litellm_spend IS 'Daily aggregate spend tracking per virtual key (virtual_key=agent identifier)';
COMMENT ON TABLE artifacts IS 'Artifact references stored in MinIO with trace correlation';
