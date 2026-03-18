import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ArrowRight, TrendingDown, Shield, Network, ClipboardList } from "lucide-react";

const YAML_AGENT_ISOLATION = `apiVersion: agentic.clawdlinux.io/v1alpha1
kind: AgentWorkload
metadata:
  name: research-run
spec:
  runtime:
    image: ghcr.io/clawdlinux/agents/browser:latest
  isolation:
    namespaceTemplate: aw-{{name}}
    serviceAccountName: research-runner
  network:
    allowFqdns:
      - github.com
      - api.openai.com
  workflow:
    steps:
      - name: crawl
        command: ["python", "entrypoint.py"]`;

const YAML_MULTI_TENANT = `apiVersion: agentic.clawdlinux.io/v1alpha1
kind: AgentWorkload
metadata:
  name: acme-research-agent
spec:
  tenant: acme
  isolation:
    namespaceTemplate: tenant-acme
  quota:
    cpu: "4"
    memory: 8Gi
  network:
    allowFqdns:
      - s3.amazonaws.com
      - api.anthropic.com
  artifacts:
    bucket: acme-agent-runs`;

const YAML_AUDIT = `apiVersion: agentic.clawdlinux.io/v1alpha1
kind: AgentWorkload
metadata:
  name: audited-run
spec:
  workflow:
    retentionDays: 30
    steps:
      - name: ingest
        command: ["python", "ingest.py"]
      - name: summarize
        command: ["python", "summarize.py"]
  audit:
    exportLogs: true
    exportPrompts: true
  artifacts:
    bucket: audit-trail`;

const yamlMap = { YAML_AGENT_ISOLATION, YAML_MULTI_TENANT, YAML_AUDIT };

const tabs = [
  {
    id: "isolation",
    label: "Agent Isolation",
    icon: Shield,
    problem:
      "Teams wire namespaces, RBAC, network policies, and storage by hand. Every new agent run becomes another copy-pasted cluster bootstrap task.",
    solution:
      "Apply one AgentWorkload manifest and let the operator provision namespace boundaries, identities, egress rules, and workflow execution automatically.",
    before: { value: "4 handoffs", label: "Namespace, RBAC, network, storage" },
    after: { value: "1 manifest", label: "Operator-managed isolation" },
    savings: { value: "Safer by default", color: "#00d4aa" },
    yamlKey: "YAML_AGENT_ISOLATION",
    configFile: "agent-isolation.yaml",
  },
  {
    id: "multi-tenant",
    label: "Multi-Tenant Clusters",
    icon: Network,
    problem:
      "Shared clusters drift fast when every tenant needs a different mix of quota, egress, artifacts, and identity rules.",
    solution:
      "Encode tenant-specific constraints in the workload spec so the controller reconciles quotas, network boundaries, and artifact storage predictably.",
    before: { value: "Ad hoc quotas", label: "Manual tenant-by-tenant setup" },
    after: { value: "Tenant profile", label: "Repeatable workload template" },
    savings: { value: "Blast radius reduced", color: "#6366f1" },
    yamlKey: "YAML_MULTI_TENANT",
    configFile: "multi-tenant.yaml",
  },
  {
    id: "audit",
    label: "Audit & Compliance",
    icon: ClipboardList,
    problem:
      "Agent logs, prompts, and outputs often disappear across pods and scripts, leaving platform teams without a trustworthy run record.",
    solution:
      "Run each workload through Argo, retain artifacts in MinIO, and export logs plus prompts as part of the workload's declared audit policy.",
    before: { value: "Fragmented logs", label: "Pods, scripts, and object storage" },
    after: { value: "Single run record", label: "Workflow, artifacts, and audit export" },
    savings: { value: "Traceability restored", color: "#f59e0b" },
    yamlKey: "YAML_AUDIT",
    configFile: "audit-trail.yaml",
  },
];

function MetricCard({ label, value, sublabel, highlight, highlightColor }) {
  return (
    <div
      className="flex-1 rounded-xl p-5"
      style={{
        background: highlight
          ? `rgba(${highlightColor}, 0.08)`
          : "rgba(255,255,255,0.03)",
        border: highlight
          ? `1px solid rgba(${highlightColor}, 0.25)`
          : "1px solid rgba(255,255,255,0.06)",
      }}
    >
      <div
        className="text-xs font-semibold uppercase tracking-widest mb-3"
        style={{
          fontFamily: "'DM Sans', sans-serif",
          color: highlight ? `rgb(${highlightColor})` : "#94a3b8",
        }}
      >
        {label}
      </div>
      <div
        className="text-2xl font-bold mb-1"
        style={{
          fontFamily: "'Syne', sans-serif",
          color: highlight ? `rgb(${highlightColor})` : "#e2e8f0",
        }}
      >
        {value}
      </div>
      <div
        className="text-sm"
        style={{
          fontFamily: "'DM Sans', sans-serif",
          color: "#94a3b8",
        }}
      >
        {sublabel}
      </div>
    </div>
  );
}

function CodeBlock({ code, title }) {
  return (
    <div
      className="rounded-xl overflow-hidden"
      style={{
        background: "rgba(5, 8, 15, 0.9)",
        border: "1px solid rgba(255,255,255,0.08)",
      }}
    >
      <div
        className="flex items-center gap-2 px-4 py-3"
        style={{
          background: "rgba(255,255,255,0.03)",
          borderBottom: "1px solid rgba(255,255,255,0.06)",
        }}
      >
        <div className="w-3 h-3 rounded-full" style={{ background: "#ff5f57" }} />
        <div className="w-3 h-3 rounded-full" style={{ background: "#ffbd2e" }} />
        <div className="w-3 h-3 rounded-full" style={{ background: "#28c840" }} />
        <span
          className="ml-2 text-xs"
          style={{ fontFamily: "'IBM Plex Mono', monospace", color: "#94a3b8" }}
        >
          {title}
        </span>
      </div>
      <pre
        className="p-5 text-xs leading-relaxed overflow-x-auto"
        style={{
          fontFamily: "'IBM Plex Mono', monospace",
          color: "#e2e8f0",
          margin: 0,
        }}
      >
        <code
          dangerouslySetInnerHTML={{
            __html: (() => {
              const escaped = code
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');
              return escaped
                .replace(/^(\s*#.*)$/gm, '<span style="color:#64748b">$1</span>')
                .replace(/"([^"]*)"/g, '<span style="color:#f59e0b">"$1"</span>')
                .replace(/\b(true|false)\b/g, '<span style="color:#f59e0b">$1</span>')
                .replace(/(apiVersion:|kind:|metadata:|name:|spec:|runtime:|image:|isolation:|namespaceTemplate:|serviceAccountName:|network:|allowFqdns:|workflow:|steps:|command:|tenant:|quota:|cpu:|memory:|artifacts:|bucket:|audit:|exportLogs:|exportPrompts:|retentionDays:)/g,
                  '<span style="color:#6366f1">$1</span>')
                .replace(/(ghcr\.io\/clawdlinux\/agents\/browser:latest)/g,
                  '<span style="color:#00d4aa">$1</span>');
            })(),
          }}
        />
      </pre>
    </div>
  );
}

export default function UseCases() {
  const [activeTab, setActiveTab] = useState(0);

  const tab = tabs[activeTab];

  return (
    <section
      id="use-cases"
      className="py-24 px-4"
      style={{
        background: "linear-gradient(180deg, #05080f 0%, #070b14 100%)",
      }}
    >
      <div className="max-w-5xl mx-auto">
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="text-center mb-14"
        >
          <div
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-semibold uppercase tracking-widest mb-6"
            style={{
              background: "rgba(99, 102, 241, 0.08)",
              border: "1px solid rgba(99, 102, 241, 0.2)",
              color: "#6366f1",
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            Use Cases
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: "#e2e8f0",
            }}
          >
            Three Ways to Deploy{" "}
            <span
              style={{
                background: "linear-gradient(135deg, #00d4aa, #6366f1)",
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
              }}
            >
              Clawdlinux Operator
            </span>
          </h2>
        </motion.div>

        {/* Tab Navigation */}
        <div
          className="flex flex-col sm:flex-row gap-1 p-1 rounded-xl mb-10"
          style={{
            background: "rgba(13, 21, 37, 0.7)",
            border: "1px solid rgba(255,255,255,0.06)",
          }}
        >
          {tabs.map((t, i) => (
            <button
              key={t.id}
              onClick={() => setActiveTab(i)}
              className="flex-1 px-4 py-3 rounded-lg text-sm font-medium transition-all duration-300 text-left sm:text-center relative"
              style={{
                fontFamily: "'DM Sans', sans-serif",
                color: activeTab === i ? "#e2e8f0" : "#94a3b8",
                background: activeTab === i ? "rgba(0, 212, 170, 0.1)" : "transparent",
                border: activeTab === i ? "1px solid rgba(0, 212, 170, 0.25)" : "1px solid transparent",
              }}
            >
              {activeTab === i && (
                <motion.span
                  layoutId="tab-indicator"
                  className="absolute inset-0 rounded-lg"
                  style={{
                    background: "rgba(0, 212, 170, 0.08)",
                  }}
                  transition={{ type: "spring", bounce: 0.2, duration: 0.5 }}
                />
              )}
              <span className="relative z-10">{t.label}</span>
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -16 }}
            transition={{ duration: 0.35, ease: "easeOut" }}
          >
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
              {/* Left: Problem / Solution / Metrics */}
              <div className="flex flex-col gap-6">
                <div
                  className="rounded-xl p-6"
                  style={{
                    background: "rgba(13, 21, 37, 0.7)",
                    border: "1px solid rgba(255,255,255,0.06)",
                  }}
                >
                  <div
                    className="text-xs font-semibold uppercase tracking-widest mb-3"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#ef4444",
                    }}
                  >
                    The Problem
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#94a3b8",
                    }}
                  >
                    {tab.problem}
                  </p>
                </div>

                <div
                  className="rounded-xl p-6"
                  style={{
                    background: "rgba(13, 21, 37, 0.7)",
                    border: "1px solid rgba(0, 212, 170, 0.15)",
                  }}
                >
                  <div
                    className="text-xs font-semibold uppercase tracking-widest mb-3"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#00d4aa",
                    }}
                  >
                    The Solution
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#e2e8f0",
                    }}
                  >
                    {tab.solution}
                  </p>
                </div>

                {/* Metrics */}
                <div className="flex gap-4">
                  <MetricCard
                    label="Before"
                    value={tab.before.value}
                    sublabel={tab.before.label}
                    highlight={false}
                  />
                  <div className="flex items-center">
                    <ArrowRight size={20} color="#94a3b8" />
                  </div>
                  <MetricCard
                    label="After"
                    value={tab.after.value}
                    sublabel={tab.after.label}
                    highlight={true}
                    highlightColor="0, 212, 170"
                  />
                </div>

                <div
                  className="rounded-xl px-5 py-4 flex items-center gap-3"
                  style={{
                    background: `rgba(${tab.savings.color === "#f59e0b" ? "245, 158, 11" : tab.savings.color === "#00d4aa" ? "0, 212, 170" : "99, 102, 241"}, 0.08)`,
                    border: `1px solid rgba(${tab.savings.color === "#f59e0b" ? "245, 158, 11" : tab.savings.color === "#00d4aa" ? "0, 212, 170" : "99, 102, 241"}, 0.2)`,
                  }}
                >
                  <TrendingDown size={18} color={tab.savings.color} />
                  <span
                    className="text-sm font-semibold"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: tab.savings.color,
                    }}
                  >
                    {tab.savings.value}
                  </span>
                </div>
              </div>

              {/* Right: Code Snippet */}
              <div>
                <CodeBlock code={yamlMap[tab.yamlKey]} title={tab.configFile} />
              </div>
            </div>
          </motion.div>
        </AnimatePresence>
      </div>
    </section>
  );
}
