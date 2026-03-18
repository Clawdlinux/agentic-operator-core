import { motion } from 'framer-motion';
import { BookOpen, Github, GitPullRequest } from 'lucide-react';

const cardStyle = {
  background: 'rgba(13,21,37,0.75)',
  border: '1px solid rgba(0,212,170,0.15)',
  backdropFilter: 'blur(16px)',
  boxShadow: '0 8px 60px rgba(0,0,0,0.5)',
};

export default function Waitlist() {
  return (
    <section
      id="waitlist"
      style={{ background: '#05080f' }}
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
          background: 'radial-gradient(circle, rgba(0,212,170,0.07) 0%, transparent 70%)',
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
              color: '#00d4aa',
              background: 'rgba(0,212,170,0.08)',
              border: '1px solid rgba(0,212,170,0.2)',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Contribute
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold text-white"
            style={{ fontFamily: "'Syne', sans-serif" }}
          >
            Build on the Open Core
          </h2>
          <p
            className="text-center text-lg mt-4 max-w-2xl mx-auto"
            style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}
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
            <Github className="w-6 h-6 mb-4" style={{ color: '#00d4aa' }} />
            <h3 className="text-lg font-semibold text-white mb-2" style={{ fontFamily: "'Syne', sans-serif" }}>
              Explore the Repo
            </h3>
            <p style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}>
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
            <BookOpen className="w-6 h-6 mb-4" style={{ color: '#00d4aa' }} />
            <h3 className="text-lg font-semibold text-white mb-2" style={{ fontFamily: "'Syne', sans-serif" }}>
              Read the Docs
            </h3>
            <p style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}>
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
            <GitPullRequest className="w-6 h-6 mb-4" style={{ color: '#00d4aa' }} />
            <h3 className="text-lg font-semibold text-white mb-2" style={{ fontFamily: "'Syne', sans-serif" }}>
              Contribute Changes
            </h3>
            <p style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}>
              Ship fixes, docs, and runtime integrations through standard GitHub review workflows.
            </p>
          </a>
        </div>
      </motion.div>
    </section>
  );
}
