# Phase 4: Agent Evaluation Pipeline

**Objective:** Implement comprehensive agent performance tracking, outcome quality evaluation, and strategic improvements based on real-world metrics.

**Timeline:** 1-2 days | **Deployment:** agentic-prod cluster

---

## Vision

Transform agents from "fire and forget" to **intelligent systems that learn and improve**:

```
Agent Task → Execution → Outcome Tracking → Evaluation → Feedback Loop → Improvement
```

**Key Metrics:**
- Task success rate (%)
- Time to completion (seconds)
- Cost per task (USD)
- Outcome quality score (0-100)
- User satisfaction (if applicable)

---

## Architecture

### Components

```yaml
┌─────────────────────────────────────────────────────────────────┐
│ Phase 4: Agent Evaluation Pipeline                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 1. Execution Tracking (Controller)                        │  │
│  │    - Task start time                                      │  │
│  │    - Agent selection                                      │  │
│  │    - Model used                                           │  │
│  │    - Input tokens / Output tokens                         │  │
│  │    - Cost estimate                                        │  │
│  └──────────────────────────────────────────────────────────┘  │
│                      ↓                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 2. Outcome Classification (New: Evaluation Service)      │  │
│  │    - Success / Failure / Partial                          │  │
│  │    - Error type (if failed)                               │  │
│  │    - Confidence in result                                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│                      ↓                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 3. Quality Scoring (ML-Free Heuristics)                  │  │
│  │    - Output length / relevance                            │  │
│  │    - Hallucination detection                              │  │
│  │    - Task satisfaction                                    │  │
│  │    - Quality Score: 0-100                                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│                      ↓                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 4. Metrics Storage (PostgreSQL)                           │  │
│  │    - agent_evaluations table                              │  │
│  │    - agent_performance_history table                      │  │
│  │    - Time-series data for trends                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                      ↓                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 5. Reporting & Analytics                                 │  │
│  │    - Prometheus metrics export                            │  │
│  │    - Grafana dashboards                                   │  │
│  │    - Weekly performance reports                           │  │
│  └──────────────────────────────────────────────────────────┘  │
│                      ↓                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 6. Feedback Loop (Operator Decision Logic)                │  │
│  │    - Promote high-performing agents                       │  │
│  │    - Demote poor performers                               │  │
│  │    - Suggest model switching                              │  │
│  │    - A/B test new strategies                              │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Plan

### Step 1: Database Schema (PostgreSQL) — 3 tables

```sql
-- Agent execution records
CREATE TABLE agent_evaluations (
  id SERIAL PRIMARY KEY,
  workload_id VARCHAR(255) NOT NULL,
  workload_namespace VARCHAR(255) NOT NULL,
  agent_name VARCHAR(255),
  model_used VARCHAR(255),
  task_category VARCHAR(50),
  
  -- Timing
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  duration_seconds INT,
  
  -- LLM usage
  input_tokens INT,
  output_tokens INT,
  estimated_cost_usd DECIMAL(10, 4),
  
  -- Outcome
  status VARCHAR(50),           -- success, failure, partial
  error_type VARCHAR(255),       -- if failed
  error_message TEXT,
  
  -- Quality
  quality_score INT,            -- 0-100
  quality_breakdown JSONB,      -- {relevance: 85, hallucination_risk: 10, ...}
  
  -- User satisfaction
  user_rating INT,              -- 1-5 (optional)
  user_feedback TEXT,
  
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Performance history (aggregated per agent, per day)
CREATE TABLE agent_performance_history (
  id SERIAL PRIMARY KEY,
  agent_name VARCHAR(255),
  metric_date DATE,
  
  -- Aggregate metrics
  total_tasks INT,
  successful_tasks INT,
  failed_tasks INT,
  success_rate DECIMAL(5, 2),   -- %
  
  avg_quality_score DECIMAL(5, 2),
  avg_duration_seconds INT,
  
  total_cost_usd DECIMAL(10, 2),
  avg_cost_per_task DECIMAL(10, 4),
  
  -- Time-series data
  performance_trend VARCHAR(50), -- improving, stable, degrading
  
  created_at TIMESTAMP DEFAULT NOW()
);

-- A/B test results
CREATE TABLE agent_ab_tests (
  id SERIAL PRIMARY KEY,
  test_name VARCHAR(255),
  start_date TIMESTAMP,
  end_date TIMESTAMP,
  
  agent_a VARCHAR(255),
  agent_b VARCHAR(255),
  
  variant_a_wins INT,
  variant_b_wins INT,
  winner VARCHAR(255),
  confidence_percent INT,
  
  created_at TIMESTAMP DEFAULT NOW()
);
```

### Step 2: Evaluation Service (Go)

**File:** `pkg/evaluation/evaluator.go` (300 lines)

```go
package evaluation

import (
    "context"
    "database/sql"
    "time"
)

// ExecutionRecord tracks a single agent task execution
type ExecutionRecord struct {
    WorkloadID      string
    Namespace       string
    AgentName       string
    ModelUsed       string
    TaskCategory    string
    
    StartedAt       time.Time
    CompletedAt     time.Time
    
    InputTokens     int
    OutputTokens    int
    EstimatedCost   float64
    
    Output          string
    Error           string
}

// QualityEvaluation scores the quality of an agent's output
type QualityEvaluation struct {
    OverallScore    int            // 0-100
    Relevance       int            // 0-100
    Hallucination   int            // 0-100 (lower is better)
    Completeness    int            // 0-100
    Clarity         int            // 0-100
    
    Details         map[string]interface{}
}

// EvaluationResult combines execution + quality
type EvaluationResult struct {
    ExecutionRecord ExecutionRecord
    Quality         QualityEvaluation
    UserRating      *int           // 1-5 (optional)
    UserFeedback    string
}

// Evaluator provides quality scoring for agent outputs
type Evaluator interface {
    // Evaluate scores the quality of agent output
    Evaluate(ctx context.Context, record ExecutionRecord) (*QualityEvaluation, error)
    
    // StoreEvaluation persists result to database
    StoreEvaluation(ctx context.Context, result *EvaluationResult) error
    
    // GetAgentStats returns performance metrics for an agent
    GetAgentStats(ctx context.Context, agentName string) (*AgentStats, error)
    
    // GetTrendAnalysis returns performance trends
    GetTrendAnalysis(ctx context.Context, agentName string, days int) (*TrendAnalysis, error)
}

// AgentStats represents aggregate performance metrics
type AgentStats struct {
    AgentName          string
    TotalTasks         int
    SuccessTasks       int
    SuccessRate        float64      // %
    AvgQualityScore    float64
    AvgCostPerTask     float64
    AvgDurationSeconds float64
}

// TrendAnalysis tracks performance over time
type TrendAnalysis struct {
    AgentName       string
    TimePeriodDays  int
    Trend           string  // improving, stable, degrading
    DailyScores     []DailyScore
}

type DailyScore struct {
    Date        time.Time
    QualityScore float64
    SuccessRate float64
    CostPerTask float64
}
```

### Step 3: Quality Scoring Logic

**File:** `pkg/evaluation/quality_scorer.go` (250 lines)

Heuristic-based scoring without ML:

```go
func (e *Evaluator) ScoreQuality(ctx context.Context, record ExecutionRecord) (*QualityEvaluation, error) {
    score := &QualityEvaluation{
        Details: make(map[string]interface{}),
    }
    
    // 1. Relevance: Did the output answer the objective?
    score.Relevance = e.scoreRelevance(record.Output, record.TaskCategory)
    
    // 2. Hallucination: Does it make up facts? Check for suspicious patterns
    score.Hallucination = e.detectHallucination(record.Output)
    
    // 3. Completeness: Is the response complete?
    score.Completeness = e.scoreCompleteness(record.Output, record.TaskCategory)
    
    // 4. Clarity: Is it well-written and clear?
    score.Clarity = e.scoreClarity(record.Output)
    
    // 5. Overall: Weighted average
    score.OverallScore = int(
        float64(score.Relevance) * 0.35 +
        float64(100 - score.Hallucination) * 0.25 +
        float64(score.Completeness) * 0.20 +
        float64(score.Clarity) * 0.20,
    )
    
    return score, nil
}

// scoreRelevance checks if output relates to task
func (e *Evaluator) scoreRelevance(output string, category string) int {
    // Check for keywords based on task type
    switch category {
    case "validation":
        // Expected: yes/no, true/false, valid/invalid
        indicators := []string{"valid", "invalid", "pass", "fail", "yes", "no"}
        return e.matchKeywords(output, indicators)
    case "analysis":
        // Expected: detailed analysis, insights, data points
        indicators := []string{"trend", "pattern", "insight", "analysis", "data"}
        return e.matchKeywords(output, indicators)
    case "reasoning":
        // Expected: reasoning chain, conclusions
        indicators := []string{"therefore", "because", "conclude", "reason", "imply"}
        return e.matchKeywords(output, indicators)
    }
    return 70 // default
}

// detectHallucination checks for made-up claims
func (e *Evaluator) detectHallucination(output string) int {
    risks := 0
    
    // Check for suspicious patterns
    suspiciousPatterns := []string{
        "I don't have access", // Then why did it answer?
        "however, I cannot",    // Contradiction
        "unfortunately, I cannot verify", // Should have said this earlier
    }
    
    for _, pattern := range suspiciousPatterns {
        if strings.Contains(strings.ToLower(output), pattern) {
            risks += 10
        }
    }
    
    // Long, overly confident responses about uncertain topics = risky
    if len(output) > 1000 && strings.Count(output, "definitely") > 3 {
        risks += 20
    }
    
    // Very short responses when detailed answer expected = risky
    if len(output) < 50 && !strings.Contains(output, "yes") {
        risks += 15
    }
    
    return min(risks, 100)
}

// scoreCompleteness checks if the answer is thorough
func (e *Evaluator) scoreCompleteness(output string, category string) int {
    length := len(output)
    
    switch category {
    case "validation":
        // Should be brief but clear
        if length < 10 {
            return 20 // Too short
        }
        if length > 500 {
            return 60 // Too verbose for a yes/no question
        }
        return 85
    case "analysis":
        // Should have substantial content
        if length < 200 {
            return 40 // Incomplete analysis
        }
        if length > 5000 {
            return 70 // Maybe overly detailed
        }
        return 90
    case "reasoning":
        // Should show thinking process
        if length < 300 {
            return 50
        }
        return 85
    }
    return 70
}

// scoreClarity checks writing quality
func (e *Evaluator) scoreClarity(output string) int {
    score := 70
    
    // Multiple typos = clarity issue
    typos := e.estimateTypos(output)
    if typos > 3 {
        score -= 20
    }
    
    // Too fragmented sentences
    sentences := len(strings.Split(output, "."))
    avgLength := len(output) / max(sentences, 1)
    if avgLength < 10 || avgLength > 100 {
        score -= 10
    }
    
    // Good structure: has some form of logical flow
    if strings.Count(output, "•") > 2 || strings.Count(output, "-") > 2 {
        score += 15
    }
    
    return min(max(score, 0), 100)
}
```

### Step 4: Controller Integration

Extend `agentworkload_controller.go` to track outcomes:

```go
// In Reconcile() method, after MCP execution:
if result.Status.Phase == "Completed" {
    // Record execution metrics
    execRecord := &evaluation.ExecutionRecord{
        WorkloadID: workload.Name,
        Namespace: workload.Namespace,
        StartedAt: workload.Status.StartTime,
        CompletedAt: time.Now(),
        Output: result.Output,
    }
    
    // Evaluate quality
    eval, err := r.evaluator.Evaluate(ctx, execRecord)
    if err == nil {
        // Store evaluation results
        r.evaluator.StoreEvaluation(ctx, &evaluation.EvaluationResult{
            ExecutionRecord: execRecord,
            Quality: eval,
        })
        
        // Update workload status with quality score
        workload.Status.QualityScore = eval.OverallScore
        r.Status().Update(ctx, workload)
    }
}
```

### Step 5: Metrics Export (Prometheus)

**File:** `pkg/metrics/evaluation_metrics.go` (150 lines)

```go
// Agent evaluation metrics
var (
    agentSuccessRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agentic_agent_success_rate",
            Help: "Success rate of agents (0-100)",
        },
        []string{"agent_name"},
    )
    
    agentQualityScore = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agentic_agent_quality_score",
            Help: "Quality score of agent outputs (0-100)",
        },
        []string{"agent_name"},
    )
    
    agentCostPerTask = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agentic_agent_cost_per_task",
            Help: "Average cost per task in USD",
        },
        []string{"agent_name"},
    )
    
    agentAvgDuration = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agentic_agent_avg_duration_seconds",
            Help: "Average task duration in seconds",
        },
        []string{"agent_name"},
    )
)
```

### Step 6: Grafana Dashboard

**File:** `config/grafana/agent-performance-dashboard.json` (400 lines)

Visualizations:
- Success rate trends (line chart)
- Quality score distribution (histogram)
- Cost per agent (bar chart)
- Duration comparison (box plot)
- Top performing agents (leaderboard)
- A/B test results comparison

---

## Test Coverage

**File:** `pkg/evaluation/evaluator_test.go` (350 lines)

```go
func TestQualityScoringValidation(t *testing.T) {
    // Good: "Yes, the input is valid"
    assert.Greater(t, scorer.scoreRelevance("Yes, the input is valid", "validation"), 80)
    
    // Bad: "I don't know"
    assert.Less(t, scorer.scoreRelevance("I don't know", "validation"), 50)
}

func TestHallucinationDetection(t *testing.T) {
    // Contradictory response = high hallucination risk
    hallRisk := scorer.detectHallucination("I don't have access to this data. However, based on my analysis...")
    assert.Greater(t, hallRisk, 15)
}

func TestQualityScoring(t *testing.T) {
    record := ExecutionRecord{
        Output: "Based on the data provided, there are three key trends: 1) Markets are rising, 2) Volatility is increasing, 3) Sector rotation is happening.",
        TaskCategory: "analysis",
    }
    quality, _ := scorer.Evaluate(context.Background(), record)
    assert.Greater(t, quality.OverallScore, 75)
}

func TestDatabaseStorage(t *testing.T) {
    result := &EvaluationResult{
        ExecutionRecord: testRecord,
        Quality: testQuality,
    }
    err := evaluator.StoreEvaluation(context.Background(), result)
    assert.NoError(t, err)
    
    // Verify stored
    stored, _ := evaluator.GetAgentStats(context.Background(), "test-agent")
    assert.Equal(t, 1, stored.TotalTasks)
}
```

---

## Deployment

### 1. Database Migration

```bash
# Apply schema
kubectl exec -n shared-services postgresql-0 -- \
  psql -U postgres -d agentic_db -f /tmp/schema.sql

# Verify
kubectl exec -n shared-services postgresql-0 -- \
  psql -U postgres -d agentic_db \
  -c "SELECT table_name FROM information_schema.tables WHERE table_schema='public';"
```

### 2. Deploy Phase 4 Code

```bash
# Build operator with Phase 4
make docker-build IMG=ghcr.io/clawdlinux/agentic-operator:phase4

# Push to registry
docker push ghcr.io/clawdlinux/agentic-operator:phase4

# Update operator deployment
kubectl set image deployment/agentic-operator \
  -n agentic-system \
  agentic-operator=ghcr.io/clawdlinux/agentic-operator:phase4
```

### 3. Deploy Grafana Dashboard

```bash
# Create ConfigMap with dashboard JSON
kubectl create configmap grafana-agent-dashboard \
  --from-file=config/grafana/agent-performance-dashboard.json \
  -n monitoring

# Reload Grafana
kubectl rollout restart deployment/grafana -n monitoring
```

---

## Success Criteria

✅ Agent execution metrics tracked (start, duration, cost)
✅ Quality scoring working (relevance, hallucination, completeness, clarity)
✅ PostgreSQL schema created + data flowing
✅ Prometheus metrics exported (success rate, quality, cost)
✅ Grafana dashboards visible and updating
✅ Agent performance trending over time
✅ A/B test infrastructure ready
✅ All tests passing (unit + integration)

---

## Next Steps

### Phase 5: Production Hardening
- Error recovery + retry logic
- Auto-scaling based on metrics
- PII scrubbing in logs
- Compliance audit trails

### Phase 6: Customer Onboarding
- Helm chart refinements
- Multi-tenant support
- Custom evaluation rules per customer
- SLA monitoring

---

**Status:** Ready to implement 🚀
