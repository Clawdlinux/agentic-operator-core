import { motion } from 'framer-motion';
import {
  Shield,
  Network,
  FileCode,
  LifeBuoy,
  ArrowRight,
  Sparkles,
  Building2,
  Lock,
  CheckCircle2,
  Boxes,
  WifiOff,
  BadgeCheck,
} from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const COMMUNITY_FEATURES = [
  'AgentWorkload CRD — full spec',
  'Cilium FQDN egress policies',
  'Argo Workflows DAG orchestration',
  'MinIO artifact retention',
  'Per-workload cost tracking and attribution',
  'Air-gapped deployment support',
  'Multi-tenant namespace isolation',
  'LiteLLM model routing',
  'Python agent runtime',
  'Apache 2.0 — self-hostable forever',
];

const ENTERPRISE_BENEFITS = [
  {
    icon: Shield,
    title: 'Managed Upgrades',
    description: 'Coordinate operator releases, CRD migrations, and rollback planning across production clusters.',
  },
  {
    icon: Network,
    title: 'Cluster Onboarding',
    description: 'Set baseline namespace, egress, identity, and storage policies with rollout support from the maintainers.',
  },
  {
    icon: FileCode,
    title: 'Workflow Design',
    description: 'Model agent runtimes, DAG steps, quotas, and artifact retention for your internal workload patterns.',
  },
  {
    icon: LifeBuoy,
    title: 'Runbook Support',
    description: 'Get help with incident response, audit retention, and day-two operations for regulated environments.',
  },
];

const ENTERPRISE_ADD_ONS = [
  {
    icon: Building2,
    title: 'Dedicated Control Planes',
    description:
      'Dedicated operator management for teams standardizing AI workloads across multiple clusters or business units.',
    color: '#8b5cf6',
  },
  {
    icon: Lock,
    title: 'Private Registry & SSO',
    description:
      'Private images, enterprise identity integration, and hardened delivery workflows for internal agent platforms.',
    color: '#06b6d4',
  },
  {
    icon: WifiOff,
    title: 'Air-Gapped & FedRAMP Overlays',
    description:
      'Deployment overlays and advisory support for FedRAMP High, HIPAA, and sovereign cloud constraints.',
    color: '#00d4aa',
  },
  {
    icon: BadgeCheck,
    title: 'SLA & Compliance Advisory',
    description:
      'Formal SLA, audit artifact support, and compliance advisory for regulated industries.',
    color: '#ec4899',
  },
];

const containerVariants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.12 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 28 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.55, ease: 'easeOut' } },
};

function BenefitRow({ benefit, currentTheme }) {
  const Icon = benefit.icon;
  return (
    <div className="flex items-start gap-4">
      <div
        className="mt-0.5 w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0"
        style={{
          background: `${currentTheme.accent.teal}1A`,
          border: `1px solid ${currentTheme.accent.teal}2E`,
        }}
      >
        <Icon size={18} color={currentTheme.accent.teal} strokeWidth={1.75} />
      </div>
      <div>
        <h4
          className="text-sm font-semibold mb-0.5"
          style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
        >
          {benefit.title}
        </h4>
        <p
          className="text-sm leading-relaxed"
          style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.tertiary }}
        >
          {benefit.description}
        </p>
      </div>
    </div>
  );
}

function AddOnCard({ product, currentTheme }) {
  const Icon = product.icon;
  return (
    <motion.div
      variants={itemVariants}
      className="relative rounded-xl p-5 transition-all duration-300 group"
      style={{
        background: `${currentTheme.bg.secondary}CC`,
        border: `1px solid ${currentTheme.border.light}`,
      }}
    >
      <div
        className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
        style={{
          background: `radial-gradient(circle at 50% 0%, ${product.color}10 0%, transparent 70%)`,
        }}
      />
      <div className="relative z-10">
        <div className="flex items-center gap-3 mb-3">
          <div
            className="w-9 h-9 rounded-lg flex items-center justify-center"
            style={{ background: `${product.color}15` }}
          >
            <Icon size={18} style={{ color: product.color }} strokeWidth={1.75} />
          </div>
          <span
            className="text-[10px] font-semibold uppercase tracking-widest px-2 py-0.5 rounded-full"
            style={{
              color: product.color,
              background: `${product.color}15`,
              border: `1px solid ${product.color}30`,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Enterprise Add-on
          </span>
        </div>
        <h4
          className="text-base font-semibold mb-1.5"
          style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
        >
          {product.title}
        </h4>
        <p
          className="text-sm leading-relaxed"
          style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.tertiary }}
        >
          {product.description}
        </p>
      </div>
    </motion.div>
  );
}

export default function Products() {
  const { currentTheme, theme } = useTheme();

  return (
    <section
      id="products"
      className="relative py-24 px-4 sm:px-6 lg:px-8 overflow-hidden"
      style={{
        background: currentTheme.bg.primary,
        transition: 'background-color 300ms ease-in-out',
      }}
    >
      {/* Decorative background glow */}
      <div
        className="absolute pointer-events-none"
        style={{
          top: '10%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: 800,
          height: 500,
          borderRadius: '50%',
          background:
            `radial-gradient(circle, ${currentTheme.accent.teal}0F 0%, ${currentTheme.accent.indigo}0A 40%, transparent 70%)`,
          filter: 'blur(60px)',
        }}
      />

      <div className="relative z-10 max-w-6xl mx-auto">
        {/* Section header */}
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: '-60px' }}
          transition={{ duration: 0.6, ease: 'easeOut' }}
          className="text-center mb-16"
        >
          <div
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-semibold uppercase tracking-widest mb-6"
            style={{
              background: `${currentTheme.accent.teal}14`,
              border: `1px solid ${currentTheme.accent.teal}33`,
              color: theme === 'dark' ? currentTheme.accent.teal : '#0b4f45',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Product Editions
            <Sparkles size={14} />
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold leading-tight mb-4"
            style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
          >
            <span
              style={{
                background: `linear-gradient(135deg, ${currentTheme.accent.teal}, ${currentTheme.accent.indigo})`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Open Source
            </span>{' '}
            &{' '}
            <span
              style={{
                background: `linear-gradient(135deg, ${currentTheme.accent.indigo}, #ec4899)`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Enterprise
            </span>
          </h2>
          <p
            className="text-base sm:text-lg max-w-2xl mx-auto"
            style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.tertiary }}
          >
            Start free with the full open-source core. Add enterprise support when you're ready for production.
          </p>
        </motion.div>

        {/* Tier comparison cards */}
        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, margin: '-60px' }}
          className="grid lg:grid-cols-2 gap-6 mb-14"
        >
          {/* Community tier */}
          <motion.div
            variants={itemVariants}
            className="rounded-2xl p-8"
            style={{
              background: `${currentTheme.bg.secondary}CC`,
              border: `1px solid ${currentTheme.border.light}`,
            }}
          >
            <div className="flex items-center gap-3 mb-2">
              <div
                className="w-10 h-10 rounded-xl flex items-center justify-center"
                style={{ background: `${currentTheme.accent.teal}1A`, border: `1px solid ${currentTheme.accent.teal}33` }}
              >
                <Boxes size={20} color={currentTheme.accent.teal} strokeWidth={1.5} />
              </div>
              <div>
                <h3 className="text-xl font-bold" style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}>
                  Community
                </h3>
                <span
                  className="text-xs font-semibold uppercase tracking-wider"
                  style={{ color: currentTheme.accent.teal, fontFamily: "'IBM Plex Mono', monospace" }}
                >
                  Free · Apache 2.0
                </span>
              </div>
            </div>

            <p className="text-sm mb-6 mt-3" style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
              Everything you need to run AI agents on Kubernetes — including full air-gapped support. Self-hostable forever.
            </p>

            <div className="space-y-2.5 mb-7">
              {COMMUNITY_FEATURES.map((feat) => (
                <div key={feat} className="flex items-center gap-2.5">
                  <CheckCircle2 size={15} style={{ color: currentTheme.accent.teal, flexShrink: 0 }} />
                  <span className="text-sm" style={{ color: currentTheme.text.secondary, fontFamily: "'DM Sans', sans-serif" }}>
                    {feat}
                  </span>
                </div>
              ))}
            </div>

            <a
              href="https://github.com/Clawdlinux/agentic-operator-core"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold rounded-xl transition-all duration-200"
              style={{
                border: `1px solid ${currentTheme.accent.teal}66`,
                color: currentTheme.accent.teal,
                background: `${currentTheme.accent.teal}10`,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = `${currentTheme.accent.teal}1A`; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = `${currentTheme.accent.teal}10`; }}
            >
              View on GitHub
              <ArrowRight size={15} />
            </a>
          </motion.div>

          {/* Enterprise tier */}
          <motion.div
            variants={itemVariants}
            className="rounded-2xl p-px"
            style={{
              background: `linear-gradient(135deg, ${currentTheme.accent.teal}59, ${currentTheme.accent.indigo}40, ${currentTheme.accent.teal}1A)`,
            }}
          >
            <div
              className="rounded-2xl p-8 h-full"
              style={{ background: currentTheme.bg.overlay }}
            >
              <div className="flex items-center gap-3 mb-2">
                <div
                  className="w-10 h-10 rounded-xl flex items-center justify-center"
                  style={{
                    background: `linear-gradient(135deg, ${currentTheme.accent.teal}33, ${currentTheme.accent.indigo}26)`,
                    border: `1px solid ${currentTheme.accent.teal}40`,
                  }}
                >
                  <Shield size={20} color={currentTheme.accent.teal} strokeWidth={1.5} />
                </div>
                <div>
                  <h3 className="text-xl font-bold" style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}>
                    Enterprise
                  </h3>
                  <span
                    className="text-xs font-semibold uppercase tracking-wider"
                    style={{ color: currentTheme.accent.teal, fontFamily: "'IBM Plex Mono', monospace" }}
                  >
                    Managed Support
                  </span>
                </div>
              </div>

              <p className="text-sm mb-6 mt-3" style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
                Everything in Community, plus expert support for production deployments — including air-gapped, FedRAMP, and sovereign cloud environments.
              </p>

              <div className="flex flex-col gap-4 mb-7">
                {ENTERPRISE_BENEFITS.map((benefit) => (
                  <BenefitRow key={benefit.title} benefit={benefit} currentTheme={currentTheme} />
                ))}
              </div>

              <p
                className="text-xs mb-4"
                style={{ color: currentTheme.text.muted, fontFamily: "'DM Sans', sans-serif" }}
              >
                Enterprise inquiries:{' '}
                <a
                  href="mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Support%20Inquiry"
                  className="underline"
                  style={{ color: currentTheme.accent.teal }}
                >
                  shreyanshsancheti09@gmail.com
                </a>
              </p>

              <a
                href="mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Support%20Inquiry"
                className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold rounded-xl transition-all duration-200 hover:brightness-110"
                style={{
                  background: `linear-gradient(135deg, ${currentTheme.accent.teal} 0%, #00b894 100%)`,
                  color: '#03231d',
                }}
              >
                Get in Touch
                <ArrowRight size={15} />
              </a>
            </div>
          </motion.div>
        </motion.div>

        {/* Enterprise add-ons */}
        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, margin: '-60px' }}
        >
          <motion.div variants={itemVariants} className="mb-5">
            <p
              className="text-xs font-semibold uppercase tracking-widest text-center"
              style={{ color: currentTheme.text.muted, fontFamily: "'IBM Plex Mono', monospace" }}
            >
              Enterprise Add-ons
            </p>
          </motion.div>
          <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {ENTERPRISE_ADD_ONS.map((product) => (
              <AddOnCard key={product.title} product={product} currentTheme={currentTheme} />
            ))}
          </div>
        </motion.div>
      </div>
    </section>
  );
}
