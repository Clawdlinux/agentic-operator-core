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
} from 'lucide-react';

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
];

const containerVariants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.12 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 28 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.55, ease: 'easeOut' } },
};

function BenefitRow({ benefit }) {
  const Icon = benefit.icon;
  return (
    <div className="flex items-start gap-4">
      <div
        className="mt-0.5 w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0"
        style={{
          background: 'rgba(0, 212, 170, 0.10)',
          border: '1px solid rgba(0, 212, 170, 0.18)',
        }}
      >
        <Icon size={18} color="#00d4aa" strokeWidth={1.75} />
      </div>
      <div>
        <h4
          className="text-sm font-semibold mb-0.5"
          style={{ fontFamily: "'Syne', sans-serif", color: '#e2e8f0' }}
        >
          {benefit.title}
        </h4>
        <p
          className="text-sm leading-relaxed"
          style={{ fontFamily: "'DM Sans', sans-serif", color: '#94a3b8' }}
        >
          {benefit.description}
        </p>
      </div>
    </div>
  );
}

function ComingSoonCard({ product }) {
  const Icon = product.icon;
  return (
    <motion.div
      variants={itemVariants}
      className="relative rounded-xl p-5 transition-all duration-300 group"
      style={{
        background: 'rgba(13, 21, 37, 0.5)',
        border: '1px solid rgba(255,255,255,0.06)',
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
          style={{ fontFamily: "'Syne', sans-serif", color: '#e2e8f0' }}
        >
          {product.title}
        </h4>
        <p
          className="text-sm leading-relaxed"
          style={{ fontFamily: "'DM Sans', sans-serif", color: '#94a3b8' }}
        >
          {product.description}
        </p>
      </div>
    </motion.div>
  );
}

export default function Products() {
  return (
    <section
      id="products"
      className="relative py-24 px-4 sm:px-6 lg:px-8 overflow-hidden"
      style={{ background: '#05080f' }}
    >
      {/* Decorative background glow */}
      <div
        className="absolute pointer-events-none"
        style={{
          top: '20%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: 800,
          height: 500,
          borderRadius: '50%',
          background:
            'radial-gradient(circle, rgba(0,212,170,0.06) 0%, rgba(99,102,241,0.04) 40%, transparent 70%)',
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
              background: 'rgba(0, 212, 170, 0.08)',
              border: '1px solid rgba(0, 212, 170, 0.2)',
              color: '#00d4aa',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Managed Offering
            <Sparkles size={14} />
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold leading-tight mb-4"
            style={{ fontFamily: "'Syne', sans-serif", color: '#e2e8f0' }}
          >
            <span
              style={{
                background: 'linear-gradient(135deg, #00d4aa, #6366f1)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Enterprise Support
            </span>
          </h2>
          <p
            className="text-base sm:text-lg max-w-2xl mx-auto"
            style={{ fontFamily: "'DM Sans', sans-serif", color: '#94a3b8' }}
          >
            For teams deploying Agentic Operator in production with managed support and hardened cluster coordination.
          </p>
        </motion.div>

          {/* Enterprise managed support offering */}
        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, margin: '-60px' }}
        >
          <motion.div
            variants={itemVariants}
            className="rounded-2xl p-px mb-10"
            style={{
              background: 'linear-gradient(135deg, rgba(0,212,170,0.35), rgba(99,102,241,0.25), rgba(0,212,170,0.1))',
            }}
          >
            <div
              className="rounded-2xl p-8 sm:p-10"
              style={{ background: 'rgba(5, 8, 15, 0.95)' }}
            >
              <div className="grid lg:grid-cols-2 gap-10 items-start">
                {/* Left: product info */}
                <div>
                  <div className="flex items-center gap-3 mb-4">
                    <div
                      className="w-12 h-12 rounded-xl flex items-center justify-center"
                      style={{
                        background: 'linear-gradient(135deg, rgba(0,212,170,0.2), rgba(99,102,241,0.15))',
                        border: '1px solid rgba(0, 212, 170, 0.25)',
                      }}
                    >
                      <Shield size={24} color="#00d4aa" strokeWidth={1.5} />
                    </div>
                    <div>
                      <h3
                        className="text-xl sm:text-2xl font-bold"
                        style={{ fontFamily: "'Syne', sans-serif", color: '#e2e8f0' }}
                      >
                        Managed Support
                      </h3>
                      <span
                        className="text-xs font-semibold uppercase tracking-wider"
                        style={{ color: '#00d4aa', fontFamily: "'IBM Plex Mono', monospace" }}
                      >
                        Enterprise
                      </span>
                    </div>
                  </div>

                  <p
                    className="text-base sm:text-lg mb-6 leading-relaxed"
                    style={{ fontFamily: "'DM Sans', sans-serif", color: '#94a3b8' }}
                  >
                    Platform teams can get expert support for deploying Agentic Operator in production with managed upgrades, workload design guidance, incident response, and hardened cluster coordination.
                  </p>

                  <p
                    className="text-sm mb-6"
                    style={{ color: '#94a3b8', fontFamily: "'DM Sans', sans-serif" }}
                  >
                    For enterprise support inquiries, reach out at{' '}
                    <a
                      href="mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Support%20Inquiry"
                      className="underline"
                      style={{ color: '#00d4aa' }}
                    >
                      shreyanshsancheti09@gmail.com
                    </a>
                    .
                  </p>

                  {/* CTA */}
                  <a
                    href="mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Support%20Inquiry"
                    className="inline-flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl transition-all duration-200 hover:brightness-110 hover:shadow-xl active:scale-[0.97]"
                    style={{
                      background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
                      color: '#05080f',
                    }}
                  >
                    Get in Touch
                    <ArrowRight size={16} />
                  </a>
                </div>

                {/* Right: benefits list */}
                <div className="flex flex-col gap-5">
                  {ENTERPRISE_BENEFITS.map((benefit) => (
                    <BenefitRow key={benefit.title} benefit={benefit} />
                  ))}
                </div>
              </div>
            </div>
          </motion.div>

          {/* Additional enterprise services */}
          <motion.div
            variants={itemVariants}
            className="mb-4"
          >
            <p
              className="text-xs font-semibold uppercase tracking-widest text-center mb-5"
              style={{ color: '#64748b', fontFamily: "'IBM Plex Mono', monospace" }}
            >
              Additional Services
            </p>
          </motion.div>
          <div className="grid sm:grid-cols-2 gap-5">
            {ENTERPRISE_ADD_ONS.map((product) => (
              <ComingSoonCard key={product.title} product={product} />
            ))}
          </div>
        </motion.div>
      </div>
    </section>
  );
}
