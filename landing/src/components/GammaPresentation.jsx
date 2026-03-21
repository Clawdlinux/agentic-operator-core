import { motion } from 'framer-motion';
import { useTheme } from '../hooks/useTheme';

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
  const { currentTheme, theme } = useTheme();
  const withAlpha = (hex, alpha) => `${hex}${alpha}`;

  return (
    <section
      id="demo"
      style={{ background: currentTheme.bg.primary }}
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
              color: currentTheme.accent.teal,
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Architecture
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: currentTheme.text.primary,
            }}
          >
            Production-Grade Design
          </h2>
        </motion.div>

        <motion.p
          variants={itemVariants}
          className="text-center text-lg mb-10"
          style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
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
                style={{ color: currentTheme.accent.teal, fontFamily: "'IBM Plex Mono', monospace" }}
              >
                {s.value}
              </span>
              <span
                className="text-sm"
                style={{ color: currentTheme.text.muted, fontFamily: "'DM Sans', sans-serif" }}
              >
                {s.label}
              </span>
            </div>
          ))}
        </motion.div>

        {/* ── v1.0.0 Features ── */}
        <motion.div variants={itemVariants}>
          <div
            className="w-full grid grid-cols-1 md:grid-cols-3 gap-4 rounded-xl p-6"
            style={{
              background:
                theme === 'dark'
                  ? withAlpha(currentTheme.bg.secondary, '80')
                  : withAlpha(currentTheme.bg.secondary, 'D9'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '1F')}`,
            }}
          >
            {[
              { icon: '🏢', title: 'Tenant CRD', desc: 'Automated multi-tenant provisioning' },
              { icon: '🔒', title: 'RBAC Isolation', desc: 'Complete namespace segregation' },
              { icon: '💰', title: 'Cost Control', desc: 'Token tracking & quota enforcement' },
              { icon: '📊', title: 'Observability', desc: 'Prometheus + OpenTelemetry' },
              { icon: '📚', title: 'Documentation', desc: '12 comprehensive guides' },
              { icon: '🚀', title: 'Production Ready', desc: '100% test coverage & validation' },
            ].map((f) => (
              <div key={f.title} className="text-center">
                <div className="text-3xl mb-2">{f.icon}</div>
                <div
                  className="text-sm font-semibold"
                  style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.primary }}
                >
                  {f.title}
                </div>
                <div
                  className="text-xs mt-1"
                  style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
                >
                  {f.desc}
                </div>
              </div>
            ))}
          </div>
        </motion.div>
      </motion.div>
    </section>
  );
}
