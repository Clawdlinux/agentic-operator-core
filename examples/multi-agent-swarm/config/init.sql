-- Multi-Agent Swarm: database initialization
-- A2A tasks + spans + OPA audit log

-- a2a_tasks: Inter-agent task delegation
CREATE TABLE IF NOT EXISTS a2a_tasks (
    id VARCHAR(36) PRIMARY KEY,
    skill VARCHAR(100) NOT NULL,
    input_data JSONB NOT NULL DEFAULT '{}',
    output_data JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    sender_agent VARCHAR(100) NOT NULL,
    recipient_agent VARCHAR(100) NOT NULL,
    error TEXT,
    timeout_seconds INT DEFAULT 300,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_a2a_tasks_recipient ON a2a_tasks(recipient_agent);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_status ON a2a_tasks(status);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_created ON a2a_tasks(created_at DESC);

-- spans_trace: Structured trace events
CREATE TABLE IF NOT EXISTS spans_trace (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    span_id VARCHAR(36) NOT NULL,
    parent_span_id VARCHAR(36),
    agent_role VARCHAR(50) NOT NULL,
    agent_tone VARCHAR(50),
    operation VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans_trace(trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_agent ON spans_trace(agent_role);

-- approval_log: Actions held / approved / denied
CREATE TABLE IF NOT EXISTS approval_log (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    agent_role VARCHAR(50) NOT NULL,
    action VARCHAR(200) NOT NULL,
    confidence DECIMAL(5, 4) NOT NULL,
    threshold DECIMAL(5, 4) NOT NULL,
    decision VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, approved, denied
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    decided_at TIMESTAMP
);

-- opa_audit: OPA policy evaluation results
CREATE TABLE IF NOT EXISTS opa_audit (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    agent_role VARCHAR(50) NOT NULL,
    policy_mode VARCHAR(20) NOT NULL,  -- strict, permissive
    action VARCHAR(200) NOT NULL,
    decision VARCHAR(10) NOT NULL,  -- allow, deny
    violations JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- litellm_spend: Aggregate spend per virtual key
CREATE TABLE IF NOT EXISTS litellm_spend (
    id SERIAL PRIMARY KEY,
    virtual_key VARCHAR(100) NOT NULL,
    total_cost_usd DECIMAL(10, 6) NOT NULL DEFAULT 0,
    total_input_tokens INT DEFAULT 0,
    total_output_tokens INT DEFAULT 0,
    request_count INT DEFAULT 0,
    period_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    period_end TIMESTAMP
);
