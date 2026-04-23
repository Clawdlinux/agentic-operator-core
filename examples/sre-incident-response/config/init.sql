-- SRE Incident Response: database initialization

-- a2a_tasks: delegation between SRE agents
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

-- incidents: Incoming alert log
CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY,
    incident_id VARCHAR(36) NOT NULL,
    alert_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    namespace VARCHAR(100),
    resource VARCHAR(200),
    triage_result JSONB,
    diagnosis_result JSONB,
    remediation_result JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    model_costs JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP
);

-- approval_log
CREATE TABLE IF NOT EXISTS approval_log (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    agent_role VARCHAR(50) NOT NULL,
    action VARCHAR(200) NOT NULL,
    confidence DECIMAL(5, 4) NOT NULL,
    threshold DECIMAL(5, 4) NOT NULL,
    decision VARCHAR(20) NOT NULL DEFAULT 'pending',
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- opa_audit
CREATE TABLE IF NOT EXISTS opa_audit (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    agent_role VARCHAR(50) NOT NULL,
    policy_mode VARCHAR(20) NOT NULL,
    action VARCHAR(200) NOT NULL,
    decision VARCHAR(10) NOT NULL,
    violations JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
