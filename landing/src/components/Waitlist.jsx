import { motion } from 'framer-motion';
import { BookOpen, Github, GitPullRequest } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

export default function Waitlist() {
  const { currentTheme, theme } = useTheme();
  const withAlpha = (hex, alpha) => `${hex}${alpha}`;

  const cardStyle = {
    background:
      theme === 'dark'
        ? withAlpha(currentTheme.bg.secondary, 'BF')
        : withAlpha(currentTheme.bg.secondary, 'E6'),
    border: `1px solid ${withAlpha(currentTheme.accent.teal, '26')}`,
    backdropFilter: 'blur(16px)',
    boxShadow: theme === 'dark' ? '0 8px 60px rgba(0,0,0,0.5)' : '0 8px 38px rgba(15,23,42,0.14)',
  };

  return (
    <section
      id="waitlist"
      style={{ background: currentTheme.bg.primary }}
      className="py-24 px-6 overflow-hidden relative"
    >
      <div
        className="absolute pointer-events-none"
        style={{
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: 600,
          height: 600,
          borderRadius: '50%',
          background: `radial-gradient(circle, ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '12' : '0D')} 0%, transparent 70%)`,
          filter: 'blur(40px)',
        }}
      />

      <motion.div
        className="max-w-4xl mx-auto relative z-10"
        initial={{ opacity: 0, y: 24 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, amount: 0.2 }}
        transition={{ duration: 0.6, ease: 'easeOut' }}
      >
        <div className="text-center mb-10">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: theme === 'dark' ? currentTheme.accent.teal : '#0b4f45',
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Contribute
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: currentTheme.text.primary,
            }}
          >
            Build on the Open Core
          </h2>
          <p
            className="text-center text-lg mt-4 max-w-2xl mx-auto"
            style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
          >
            Star the repo, read the docs, or open a PR. The landing page no longer runs a waitlist flow; the next step is shipping code.
          </p>
        </div>

        <div className="grid md:grid-cols-3 gap-5">
          <a
            href="https://github.com/Clawdlinux/agentic-operator-core"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-2xl p-6 transition-all duration-200 hover:-translate-y-1"
            style={cardStyle}
          >
            <Github className="w-6 h-6 mb-4" style={{ color: currentTheme.accent.teal }} />
            <h3
              className="text-lg font-semibold mb-2"
              style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
            >
              Explore the Repo
            </h3>
            <p style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
              Review CRDs, controllers, agents, and deployment assets directly in GitHub.
            </p>
          </a>

          <a
            href="https://github.com/Clawdlinux/agentic-operator-core/tree/main/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-2xl p-6 transition-all duration-200 hover:-translate-y-1"
            style={cardStyle}
          >
            <BookOpen className="w-6 h-6 mb-4" style={{ color: currentTheme.accent.teal }} />
            <h3
              className="text-lg font-semibold mb-2"
              style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
            >
              Read the Docs
            </h3>
            <p style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
              Start with installation, architecture, and multi-tenancy guidance before deploying.
            </p>
          </a>

          <a
            href="https://github.com/Clawdlinux/agentic-operator-core/pulls"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-2xl p-6 transition-all duration-200 hover:-translate-y-1"
            style={cardStyle}
          >
            <GitPullRequest className="w-6 h-6 mb-4" style={{ color: currentTheme.accent.teal }} />
            <h3
              className="text-lg font-semibold mb-2"
              style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
            >
              Contribute Changes
            </h3>
            <p style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
              Ship fixes, docs, and runtime integrations through standard GitHub review workflows.
            </p>
          </a>
        </div>
      </motion.div>
    </section>
  );
}
