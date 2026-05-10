import { motion } from 'framer-motion';
import {
  Boxes,
  Code2,
  ArrowRight,
  CheckCircle2,
  TrendingDown,
  Github,
} from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const ACL_GITHUB = 'https://github.com/Clawdlinux/ninevigil-acp';
const OPERATOR_GITHUB = 'https://github.com/Clawdlinux/agentic-operator-core';

const PRODUCTS = [
  {
    badge: 'PRODUCT 01',
    icon: Boxes,
    name: 'Agentic Operator',
    tag: 'Run agents in Kubernetes',
    summary:
      'Zero-egress, FedRAMP-ready operator that turns kubectl apply into a sandboxed, observable, cost-attributed agent workload. Argo DAGs, Cilium policy, OPA guardrails, per-tenant budgets — production from day one.',
    highlights: [
      { icon: CheckCircle2, text: 'AgentWorkload, AgentCard, Tenant CRDs' },
      { icon: CheckCircle2, text: 'Cilium FQDN egress + OPA admission' },
      { icon: CheckCircle2, text: 'Argo Workflows DAG orchestration' },
      { icon: CheckCircle2, text: 'Per-workload OpenMeter cost attribution' },
    ],
    cta: { label: 'Operator on GitHub', href: OPERATOR_GITHUB },
    accentKey: 'teal',
  },
  {
    badge: 'PRODUCT 02 · NEW',
    icon: Code2,
    name: 'ACL — Agent Context Language',
    tag: 'Feed agents 90% fewer tokens',
    summary:
      'A compact, machine-native representation of structured data, designed for LLM agents instead of humans. Three translators ship today (Kubernetes, OpenAPI, Postgres). Same fact-extraction accuracy at one-tenth the prompt tokens, validated on a 1,620-trial Anthropic benchmark.',
    highlights: [
      { icon: TrendingDown, text: '132× on live K8s namespace' },
      { icon: TrendingDown, text: '68× on the GitHub OpenAPI spec' },
      { icon: TrendingDown, text: '3.5× on realistic pg_dump output' },
      { icon: CheckCircle2, text: 'Spec CC BY 4.0 · SDKs Apache 2.0 · CLI + Python decoder' },
    ],
    cta: { label: 'ACL on GitHub', href: ACL_GITHUB },
    accentKey: 'teal',
  },
];

const SHARED_THESIS = [
  {
    title: 'Same audience',
    detail:
      'Platform teams shipping agents into regulated environments — finance, healthcare, government, sovereign cloud.',
  },
  {
    title: 'Same thesis',
    detail:
      'Agents need infrastructure that was designed for them, not retrofitted from human-shaped tooling.',
  },
  {
    title: 'Same control plane',
    detail:
      'ACL is the data layer the Operator pre-injects into every Execution Manifest. Two products, one stack.',
  },
];

export default function ProductsAcl() {
  const { currentTheme } = useTheme();
  const t = currentTheme;

  return (
    <section
      id="products"
      style={{
        padding: '100px 24px',
        maxWidth: 1200,
        margin: '0 auto',
      }}
    >
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.6 }}
        style={{ textAlign: 'center', marginBottom: 56 }}
      >
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 8,
            padding: '6px 16px',
            borderRadius: 20,
            background: `${t.accent.teal}15`,
            border: `1px solid ${t.accent.teal}30`,
            marginBottom: 20,
          }}
        >
          <span
            style={{
              color: t.accent.teal,
              fontSize: 13,
              fontWeight: 600,
              fontFamily: 'IBM Plex Mono, monospace',
            }}
          >
            TWO PRODUCTS · ONE STACK
          </span>
        </div>
        <h2
          style={{
            fontFamily: 'Syne, sans-serif',
            fontSize: 'clamp(28px, 4vw, 44px)',
            fontWeight: 800,
            color: t.text.primary,
            margin: '0 0 16px',
            lineHeight: 1.15,
          }}
        >
          Run agents. Feed agents.
          <br />
          <span style={{ color: t.accent.teal }}>Same problem, two layers.</span>
        </h2>
        <p
          style={{
            fontSize: 18,
            color: t.text.secondary,
            maxWidth: 720,
            margin: '0 auto',
            lineHeight: 1.6,
          }}
        >
          NineVigil ships the operational layer (Kubernetes operator) and the
          data layer (compact agent-native representation format) for
          production AI agents in regulated environments.
        </p>
      </motion.div>

      {/* Two product cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))',
          gap: 24,
          marginBottom: 64,
        }}
      >
        {PRODUCTS.map((p, i) => {
          const Icon = p.icon;
          return (
            <motion.a
              key={p.name}
              href={p.cta.href}
              target="_blank"
              rel="noreferrer"
              initial={{ opacity: 0, y: 24 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.5, delay: i * 0.1 }}
              whileHover={{ y: -4, transition: { duration: 0.2 } }}
              style={{
                display: 'flex',
                flexDirection: 'column',
                padding: '32px 28px',
                borderRadius: 16,
                background: t.bg.card,
                border: `1px solid ${t.border.default}`,
                textDecoration: 'none',
                color: 'inherit',
                cursor: 'pointer',
                transition: 'border-color 0.2s, transform 0.2s',
              }}
            >
              {/* Badge */}
              <div
                style={{
                  display: 'inline-block',
                  alignSelf: 'flex-start',
                  padding: '4px 10px',
                  borderRadius: 6,
                  background: `${t.accent.teal}15`,
                  color: t.accent.teal,
                  fontSize: 11,
                  fontWeight: 600,
                  fontFamily: 'IBM Plex Mono, monospace',
                  letterSpacing: 0.5,
                  marginBottom: 20,
                }}
              >
                {p.badge}
              </div>

              {/* Icon + name */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 14,
                  marginBottom: 8,
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 44,
                    height: 44,
                    borderRadius: 10,
                    background: `${t.accent.teal}20`,
                  }}
                >
                  <Icon size={22} style={{ color: t.accent.teal }} />
                </div>
                <div>
                  <h3
                    style={{
                      fontFamily: 'Syne, sans-serif',
                      fontSize: 22,
                      fontWeight: 700,
                      color: t.text.primary,
                      margin: 0,
                      lineHeight: 1.2,
                    }}
                  >
                    {p.name}
                  </h3>
                </div>
              </div>

              <p
                style={{
                  fontSize: 14,
                  color: t.text.secondary,
                  fontFamily: 'IBM Plex Mono, monospace',
                  margin: '0 0 16px',
                }}
              >
                {p.tag}
              </p>

              <p
                style={{
                  fontSize: 15,
                  color: t.text.secondary,
                  lineHeight: 1.65,
                  margin: '0 0 24px',
                }}
              >
                {p.summary}
              </p>

              {/* Highlights */}
              <ul
                style={{
                  listStyle: 'none',
                  padding: 0,
                  margin: '0 0 28px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 10,
                }}
              >
                {p.highlights.map((h) => {
                  const HIcon = h.icon;
                  return (
                    <li
                      key={h.text}
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 10,
                        fontSize: 14,
                        color: t.text.primary,
                        lineHeight: 1.5,
                      }}
                    >
                      <HIcon
                        size={16}
                        style={{
                          color: t.accent.teal,
                          flexShrink: 0,
                          marginTop: 2,
                        }}
                      />
                      <span>{h.text}</span>
                    </li>
                  );
                })}
              </ul>

              {/* CTA */}
              <div
                style={{
                  marginTop: 'auto',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 8,
                  fontSize: 14,
                  fontWeight: 600,
                  color: t.accent.teal,
                  fontFamily: 'IBM Plex Mono, monospace',
                }}
              >
                <Github size={16} />
                {p.cta.label}
                <ArrowRight size={14} />
              </div>
            </motion.a>
          );
        })}
      </div>

      {/* Shared thesis row */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.6 }}
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
          gap: 20,
          padding: '32px',
          borderRadius: 16,
          background: t.bg.card,
          border: `1px solid ${t.border.default}`,
        }}
      >
        {SHARED_THESIS.map((s) => (
          <div key={s.title}>
            <h4
              style={{
                fontFamily: 'Syne, sans-serif',
                fontSize: 16,
                fontWeight: 700,
                color: t.accent.teal,
                margin: '0 0 8px',
              }}
            >
              {s.title}
            </h4>
            <p
              style={{
                fontSize: 14,
                color: t.text.secondary,
                lineHeight: 1.6,
                margin: 0,
              }}
            >
              {s.detail}
            </p>
          </div>
        ))}
      </motion.div>
    </section>
  );
}
