import { useState } from 'react';
import { motion } from 'framer-motion';
import { Github, GitFork, Star, Clipboard, Check } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const containerVariants = {
  hidden: { opacity: 0, y: 40 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.7, ease: 'easeOut', staggerChildren: 0.15 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 24 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.6, ease: 'easeOut' } },
};

const helmCommand = `git clone https://github.com/Clawdlinux/agentic-operator-core.git`;

const badges = [
  { label: 'Apache 2.0 License', color: '#00d4aa', bg: 'rgba(0,212,170,0.1)', border: 'rgba(0,212,170,0.25)' },
  { label: 'Go + Kubernetes', color: '#6366f1', bg: 'rgba(99,102,241,0.1)', border: 'rgba(99,102,241,0.25)' },
  { label: 'Controller Runtime', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)', border: 'rgba(245,158,11,0.25)' },
];

const contributionStats = [
  { value: 'Open', label: 'Source Repository' },
  { value: 'K8s', label: 'Operator-Native' },
  { value: 'Apache', label: '2.0 License' },
];

const STARTER_TEMPLATES = [
  {
    name: 'AgentWorkload Example',
    description: 'Baseline workload manifest to validate your cluster setup and reconciliation flow.',
    href: 'https://github.com/Clawdlinux/agentic-operator-core/blob/main/config/agentworkload_example.yaml',
  },
  {
    name: 'Cost-Aware Routing',
    description: 'Model mapping template for validation, analysis, and reasoning task routing.',
    href: 'https://github.com/Clawdlinux/agentic-operator-core/blob/main/config/samples/agentworkload-cost-aware-routing.yaml',
  },
  {
    name: 'Hedge Fund Demo',
    description: 'Realistic multi-step workflow sample with artifacts and orchestration controls.',
    href: 'https://github.com/Clawdlinux/agentic-operator-core/blob/main/config/samples/hedge-fund-demo.yaml',
  },
];

export default function OpenSource() {
  const [copied, setCopied] = useState(false);
  const { currentTheme, theme } = useTheme();
  const withAlpha = (hex, alpha) => `${hex}${alpha}`;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(helmCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback: create a textarea
      const el = document.createElement('textarea');
      el.value = helmCommand;
      document.body.appendChild(el);
      el.select();
      document.execCommand('copy');
      document.body.removeChild(el);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <section
      id="github"
      style={{ background: currentTheme.bg.primary }}
      className="py-24 px-6 overflow-hidden"
    >
      {/* Subtle top divider */}
      <div
        className="max-w-5xl mx-auto mb-0"
        style={{ borderTop: `1px solid ${withAlpha(currentTheme.accent.teal, '14')}` }}
      />

      <motion.div
        className="max-w-4xl mx-auto pt-16"
        variants={containerVariants}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, amount: 0.2 }}
      >
        {/* GitHub icon */}
        <motion.div variants={itemVariants} className="flex justify-center mb-6">
          <div
            className="w-20 h-20 rounded-2xl flex items-center justify-center"
            style={{
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              boxShadow: `0 0 40px ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '1F' : '14')}`,
            }}
          >
            <Github size={42} style={{ color: currentTheme.accent.teal }} strokeWidth={1.5} />
          </div>
        </motion.div>

        {/* Section heading */}
        <motion.div variants={itemVariants} className="text-center mb-3">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: currentTheme.accent.teal,
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Open Source
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: currentTheme.text.primary,
            }}
          >
            Powered by Open Source
          </h2>
        </motion.div>

        <motion.p
          variants={itemVariants}
          className="text-center text-lg mb-12"
          style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
        >
          Agentic Operator is open source for policy-aware AI workloads on Kubernetes. Inspect, extend, and self-host.
        </motion.p>

        {/* Repo card */}
        <motion.div
          variants={itemVariants}
          className="rounded-2xl p-6 mb-8"
          style={{
            background:
              theme === 'dark'
                ? withAlpha(currentTheme.bg.secondary, 'B3')
                : withAlpha(currentTheme.bg.secondary, 'E6'),
            border: `1px solid ${withAlpha(currentTheme.accent.teal, '26')}`,
            backdropFilter: 'blur(12px)',
            boxShadow: theme === 'dark' ? '0 4px 40px rgba(0,0,0,0.4)' : '0 4px 24px rgba(15,23,42,0.12)',
          }}
        >
          {/* Repo header */}
          <div className="flex items-start justify-between flex-wrap gap-4 mb-4">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <Github size={18} style={{ color: currentTheme.text.tertiary }} />
                <a
                  href="https://github.com/Clawdlinux/agentic-operator-core"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-semibold text-lg hover:underline"
                  style={{
                    color: currentTheme.accent.teal,
                    fontFamily: "'IBM Plex Mono', monospace",
                    textDecoration: 'none',
                  }}
                >
                  Clawdlinux/agentic-operator-core
                </a>
              </div>
              <p
                className="text-sm"
                style={{
                  color: currentTheme.text.tertiary,
                  fontFamily: "'DM Sans', sans-serif",
                }}
              >
                Open-source Kubernetes operator for running and orchestrating autonomous AI workloads.
              </p>
            </div>

            {/* CTA buttons */}
            <div className="flex items-center gap-3 flex-wrap">
              <a
                href="https://github.com/Clawdlinux/agentic-operator-core"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-200 hover:scale-105"
                style={{
                  background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '1F' : '14'),
                  border: `1px solid ${withAlpha(currentTheme.accent.teal, '4D')}`,
                  color: theme === 'dark' ? currentTheme.accent.teal : '#0b4f45',
                  fontFamily: "'DM Sans', sans-serif",
                  textDecoration: 'none',
                }}
              >
                <Star size={15} />
                Star on GitHub
              </a>
              <a
                href="https://github.com/Clawdlinux/agentic-operator-core/fork"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-200 hover:scale-105"
                style={{
                  background: withAlpha(currentTheme.accent.indigo, theme === 'dark' ? '1F' : '14'),
                  border: `1px solid ${withAlpha(currentTheme.accent.indigo, '4D')}`,
                  color: theme === 'dark' ? currentTheme.accent.indigo : '#2b3672',
                  fontFamily: "'DM Sans', sans-serif",
                  textDecoration: 'none',
                }}
              >
                <GitFork size={15} />
                Fork
              </a>
            </div>
          </div>

          {/* Badges */}
          <div className="flex flex-wrap gap-2">
            {badges.map((badge) => (
              <span
                key={badge.label}
                className="inline-block text-xs font-semibold px-2.5 py-1 rounded-full"
                style={{
                  color: badge.color,
                  background: badge.bg,
                  border: `1px solid ${badge.border}`,
                  fontFamily: "'IBM Plex Mono', monospace",
                }}
              >
                {badge.label}
              </span>
            ))}
          </div>
        </motion.div>

        {/* Helm install command */}
        <motion.div variants={itemVariants} className="mb-12">
          <p
            className="text-sm font-semibold mb-3"
            style={{
              color: currentTheme.text.tertiary,
              fontFamily: "'IBM Plex Mono', monospace",
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
            }}
          >
            Quick Start
          </p>
          <div
            className="relative rounded-xl p-5"
            style={{
              background: theme === 'dark' ? '#0a0f1e' : '#f8fafc',
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '26')}`,
              boxShadow:
                theme === 'dark'
                  ? 'inset 0 1px 0 rgba(255,255,255,0.03)'
                  : 'inset 0 1px 0 rgba(15,23,42,0.03)',
            }}
          >
            {/* Terminal dots */}
            <div className="flex items-center gap-1.5 mb-4">
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#ff5f57' }} />
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#febc2e' }} />
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#28c840' }} />
            </div>

            <pre
              className="text-sm leading-relaxed overflow-x-auto"
              style={{
                fontFamily: "'IBM Plex Mono', monospace",
                color: currentTheme.text.primary,
                margin: 0,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              <span style={{ color: currentTheme.accent.teal }}>$</span>{' '}
              {`git clone https://github.com/Clawdlinux/agentic-operator-core.git`}
            </pre>

            {/* Copy button */}
            <button
              onClick={handleCopy}
              className="absolute top-4 right-4 flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 hover:scale-105"
              style={{
                background: copied
                  ? withAlpha(currentTheme.accent.teal, theme === 'dark' ? '26' : '18')
                  : withAlpha(currentTheme.bg.secondary, theme === 'dark' ? '8C' : 'CC'),
                border: copied
                  ? `1px solid ${withAlpha(currentTheme.accent.teal, '66')}`
                  : `1px solid ${currentTheme.border.light}`,
                color: copied ? currentTheme.accent.teal : currentTheme.text.tertiary,
                cursor: 'pointer',
                fontFamily: "'IBM Plex Mono', monospace",
              }}
            >
              {copied ? (
                <>
                  <Check size={12} />
                  Copied
                </>
              ) : (
                <>
                  <Clipboard size={12} />
                  Copy
                </>
              )}
            </button>
          </div>
        </motion.div>

        {/* Starter templates */}
        <motion.div variants={itemVariants} className="mb-12">
          <p
            className="text-sm font-semibold mb-4"
            style={{
              color: currentTheme.text.tertiary,
              fontFamily: "'IBM Plex Mono', monospace",
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
            }}
          >
            Starter Templates
          </p>
          <div className="grid md:grid-cols-3 gap-4">
            {STARTER_TEMPLATES.map((template) => (
              <a
                key={template.name}
                href={template.href}
                target="_blank"
                rel="noopener noreferrer"
                className="rounded-xl p-4 transition-all duration-200 hover:-translate-y-1"
                style={{
                  background:
                    theme === 'dark'
                      ? withAlpha(currentTheme.bg.secondary, 'A6')
                      : withAlpha(currentTheme.bg.secondary, 'E6'),
                  border: `1px solid ${currentTheme.border.light}`,
                  textDecoration: 'none',
                }}
              >
                <h4
                  className="text-sm font-semibold mb-2"
                  style={{ color: currentTheme.text.primary, fontFamily: "'Syne', sans-serif" }}
                >
                  {template.name}
                </h4>
                <p
                  className="text-xs leading-relaxed"
                  style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
                >
                  {template.description}
                </p>
              </a>
            ))}
          </div>
        </motion.div>

        {/* Contribution stats */}
        <motion.div
          variants={itemVariants}
          className="grid grid-cols-3 gap-4"
        >
          {contributionStats.map((stat) => (
            <div
              key={stat.label}
              className="text-center rounded-xl py-6 px-4"
              style={{
                background:
                  theme === 'dark'
                    ? withAlpha(currentTheme.bg.secondary, '80')
                    : withAlpha(currentTheme.bg.secondary, 'D9'),
                border: `1px solid ${withAlpha(currentTheme.accent.teal, '1F')}`,
              }}
            >
              <div
                className="text-2xl font-bold mb-1"
                style={{
                  color: currentTheme.accent.teal,
                  fontFamily: "'Syne', sans-serif",
                }}
              >
                {stat.value}
              </div>
              <div
                className="text-sm"
                style={{
                  color: currentTheme.text.tertiary,
                  fontFamily: "'DM Sans', sans-serif",
                }}
              >
                {stat.label}
              </div>
            </div>
          ))}
        </motion.div>
      </motion.div>
    </section>
  );
}
