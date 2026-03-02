import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Presentation, Code } from 'lucide-react';

const containerVariants = {
  hidden: { opacity: 0, y: 40 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.7, ease: 'easeOut', staggerChildren: 0.18 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 24 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.6, ease: 'easeOut' } },
};

export default function GammaPresentation() {
  const [showDeck, setShowDeck] = useState(true);

  return (
    <section
      id="demo"
      style={{ background: '#05080f' }}
      className="py-24 px-6 overflow-hidden"
    >
      <motion.div
        className="max-w-4xl mx-auto"
        variants={containerVariants}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, amount: 0.15 }}
      >
        {/* ── Header ── */}
        <motion.div variants={itemVariants} className="text-center mb-4">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: '#00d4aa',
              background: 'rgba(0,212,170,0.08)',
              border: '1px solid rgba(0,212,170,0.2)',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Architecture
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold text-white"
            style={{ fontFamily: "'Syne', sans-serif" }}
          >
            Production-Grade Design
          </h2>
        </motion.div>

        <motion.p
          variants={itemVariants}
          className="text-center text-lg mb-10"
          style={{ color: 'rgba(255,255,255,0.5)', fontFamily: "'DM Sans', sans-serif" }}
        >
          Enterprise Kubernetes operator with autonomous multi-agent consensus on agentic-prod cluster.
        </motion.p>

        {/* ── Quick stats ── */}
        <motion.div
          variants={itemVariants}
          className="flex flex-wrap items-center justify-center gap-6 mt-8 mb-16"
        >
          {[
            { value: '47/47', label: 'Pods Healthy' },
            { value: '100%', label: 'Uptime (72h)' },
            { value: '$82-90', label: 'Monthly Cost' },
          ].map((s) => (
            <div key={s.label} className="flex items-center gap-2">
              <span
                className="text-sm font-semibold"
                style={{ color: '#00d4aa', fontFamily: "'IBM Plex Mono', monospace" }}
              >
                {s.value}
              </span>
              <span
                className="text-sm"
                style={{ color: 'rgba(255,255,255,0.35)', fontFamily: "'DM Sans', sans-serif" }}
              >
                {s.label}
              </span>
            </div>
          ))}
        </motion.div>

        {/* ── Pitch deck (collapsible) ── */}
        <motion.div variants={itemVariants}>
          <button
            onClick={() => setShowDeck((v) => !v)}
            className="w-full flex items-center justify-between px-5 py-4 rounded-xl transition-all duration-200 group"
            style={{
              background: 'rgba(13,21,37,0.5)',
              border: '1px solid rgba(0,212,170,0.12)',
              cursor: 'pointer',
            }}
          >
            <div className="flex items-center gap-3">
              <div
                className="w-8 h-8 rounded-lg flex items-center justify-center"
                style={{ background: 'rgba(0,212,170,0.1)', border: '1px solid rgba(0,212,170,0.2)' }}
              >
                <Presentation size={15} style={{ color: '#00d4aa' }} />
              </div>
              <div className="text-left">
                <div
                  className="text-sm font-semibold text-white"
                  style={{ fontFamily: "'DM Sans', sans-serif" }}
                >
                  Business Overview
                </div>
                <div
                  className="text-xs mt-0.5"
                  style={{ color: 'rgba(255,255,255,0.4)', fontFamily: "'DM Sans', sans-serif" }}
                >
                  Vision · Strategy · Roadmap
                </div>
              </div>
            </div>
            <motion.div
              animate={{ rotate: showDeck ? 180 : 0 }}
              transition={{ duration: 0.25 }}
              style={{ color: 'rgba(0,212,170,0.6)' }}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path d="M4 6l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </motion.div>
          </button>

          <AnimatePresence initial={false}>
            {showDeck && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.35, ease: 'easeInOut' }}
                style={{ overflow: 'hidden' }}
              >
                <div className="pt-4 relative">
                  {/* Ambient glow behind iframe */}
                  <div
                    className="absolute inset-0 rounded-full blur-3xl pointer-events-none"
                    style={{ background: 'rgba(0,212,170,0.05)' }}
                  />
                  <div
                    className="rounded-xl overflow-hidden"
                    style={{
                      position: 'relative',
                      border: '1px solid rgba(0,212,170,0.18)',
                      boxShadow: '0 8px 40px rgba(0,0,0,0.4)',
                    }}
                  >
                    <iframe
                      src="https://gamma.app/embed/g53pjztg8z13h71"
                      style={{
                        width: '100%',
                        height: 500,
                        display: 'block',
                        border: 'none',
                      }}
                      allow="fullscreen"
                      title="Agentic Operator — Business Overview"
                      loading="lazy"
                    />
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </motion.div>
    </section>
  );
}
