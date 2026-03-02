import { useState } from 'react';
import { motion } from 'framer-motion';
import { Github, GitFork, Star, Clipboard, Check } from 'lucide-react';

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

const helmCommand = `helm install agentic \\
  oci://registry.digitalocean.com/agentic-operator/charts/agentic-operator \\
  --namespace agentic-system --create-namespace`;

const badges = [
  { label: 'Apache 2.0', color: '#00d4aa', bg: 'rgba(0,212,170,0.1)', border: 'rgba(0,212,170,0.25)' },
  { label: 'Go 1.25', color: '#6366f1', bg: 'rgba(99,102,241,0.1)', border: 'rgba(99,102,241,0.25)' },
  { label: 'Kubebuilder v4', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)', border: 'rgba(245,158,11,0.25)' },
];

const contributionStats = [
  { value: '47', label: 'Go Tests' },
  { value: '7', label: 'Python Tests' },
  { value: '5 Weeks', label: 'to Production' },
];

export default function OpenSource() {
  const [copied, setCopied] = useState(false);

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
      style={{ background: '#05080f' }}
      className="py-24 px-6 overflow-hidden"
    >
      {/* Subtle top divider */}
      <div
        className="max-w-5xl mx-auto mb-0"
        style={{ borderTop: '1px solid rgba(0,212,170,0.08)' }}
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
              background: 'rgba(0,212,170,0.08)',
              border: '1px solid rgba(0,212,170,0.2)',
              boxShadow: '0 0 40px rgba(0,212,170,0.12)',
            }}
          >
            <Github size={42} style={{ color: '#00d4aa' }} strokeWidth={1.5} />
          </div>
        </motion.div>

        {/* Section heading */}
        <motion.div variants={itemVariants} className="text-center mb-3">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: '#00d4aa',
              background: 'rgba(0,212,170,0.08)',
              border: '1px solid rgba(0,212,170,0.2)',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Open Source
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold text-white"
            style={{ fontFamily: "'Syne', sans-serif" }}
          >
            Built in the Open
          </h2>
        </motion.div>

        <motion.p
          variants={itemVariants}
          className="text-center text-lg mb-12"
          style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}
        >
          Apache 2.0 licensed. Star it, fork it, contribute.
        </motion.p>

        {/* Repo card */}
        <motion.div
          variants={itemVariants}
          className="rounded-2xl p-6 mb-8"
          style={{
            background: 'rgba(13,21,37,0.7)',
            border: '1px solid rgba(0,212,170,0.15)',
            backdropFilter: 'blur(12px)',
            boxShadow: '0 4px 40px rgba(0,0,0,0.4)',
          }}
        >
          {/* Repo header */}
          <div className="flex items-start justify-between flex-wrap gap-4 mb-4">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <Github size={18} style={{ color: 'rgba(255,255,255,0.5)' }} />
                <a
                  href="https://github.com/Clawdlinux/agentic-operator-core"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-semibold text-lg hover:underline"
                  style={{
                    color: '#00d4aa',
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
                  color: 'rgba(255,255,255,0.55)',
                  fontFamily: "'DM Sans', sans-serif",
                }}
              >
                Production-grade Kubernetes operator for orchestrating AI agent workloads
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
                  background: 'rgba(0,212,170,0.12)',
                  border: '1px solid rgba(0,212,170,0.3)',
                  color: '#00d4aa',
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
                  background: 'rgba(99,102,241,0.12)',
                  border: '1px solid rgba(99,102,241,0.3)',
                  color: '#6366f1',
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
              color: 'rgba(255,255,255,0.5)',
              fontFamily: "'IBM Plex Mono', monospace",
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
            }}
          >
            Install via Helm
          </p>
          <div
            className="relative rounded-xl p-5"
            style={{
              background: '#0a0f1e',
              border: '1px solid rgba(0,212,170,0.15)',
              boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.03)',
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
                color: '#e2e8f0',
                margin: 0,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              <span style={{ color: '#00d4aa' }}>$</span>{' '}
              {`helm install agentic \\
  oci://registry.digitalocean.com/agentic-operator/charts/agentic-operator \\
  --namespace agentic-system --create-namespace`}
            </pre>

            {/* Copy button */}
            <button
              onClick={handleCopy}
              className="absolute top-4 right-4 flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 hover:scale-105"
              style={{
                background: copied ? 'rgba(0,212,170,0.15)' : 'rgba(255,255,255,0.05)',
                border: copied ? '1px solid rgba(0,212,170,0.4)' : '1px solid rgba(255,255,255,0.1)',
                color: copied ? '#00d4aa' : 'rgba(255,255,255,0.5)',
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
                background: 'rgba(13,21,37,0.5)',
                border: '1px solid rgba(0,212,170,0.1)',
              }}
            >
              <div
                className="text-2xl font-bold mb-1"
                style={{
                  color: '#00d4aa',
                  fontFamily: "'Syne', sans-serif",
                }}
              >
                {stat.value}
              </div>
              <div
                className="text-sm"
                style={{
                  color: 'rgba(255,255,255,0.5)',
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
