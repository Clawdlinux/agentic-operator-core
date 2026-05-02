import React from 'react';
import { motion } from 'framer-motion';
import { CheckCircle2, XCircle, Zap, AlertTriangle } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const Comparison = () => {
  const { currentTheme } = useTheme();

  const containerVariants = {
    hidden: {},
    visible: {
      transition: {
        staggerChildren: 0.12,
        delayChildren: 0.1,
      },
    },
  };

  const itemVariants = {
    hidden: { opacity: 0, y: 28 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.55, ease: 'easeOut' },
    },
  };

  const headerVariants = {
    hidden: { opacity: 0, y: -20 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.6, ease: 'easeOut' },
    },
  };

  const features = [
    {
      name: 'Air-Gapped Deployment',
      agentic: 'Fully offline — no egress, no callbacks',
      kagent: 'Cloud-connected enterprise tier required',
      agenticWins: true,
      description: 'FedRAMP · HIPAA · Sovereign cloud',
    },
    {
      name: 'Outcome-Based Billing',
      agentic: 'Per-workload cost tracking and budget controls',
      kagent: 'No cost metering or budget enforcement',
      agenticWins: true,
      description: 'Chargeback to business units',
    },
    {
      name: 'DAG Workflow Orchestration',
      agentic: 'Argo Workflows — fan-out, retries, DAGs',
      kagent: 'Agent runtime only, no DAG equivalent',
      agenticWins: true,
      description: 'Multi-step agent pipelines',
    },
    {
      name: 'Per-Tenant Cost Isolation',
      agentic: 'Namespace quota + token budget per tenant',
      kagent: 'No tenant-level cost attribution',
      agenticWins: true,
      description: 'Multi-tenant SaaS & regulated orgs',
    },
    {
      name: 'Open Source Core',
      agentic: 'Apache 2.0 — self-hostable, air-gapped',
      kagent: 'CNCF Sandbox, enterprise tier is SaaS',
      agenticWins: true,
      description: 'Zero vendor lock-in',
    },
  ];

  return (
    <section
      id="comparison"
      className="py-24 px-6 overflow-hidden transition-colors duration-300"
      style={{
        background: `linear-gradient(to bottom, ${currentTheme.bg.primary}, ${currentTheme.bg.secondary})`,
      }}
    >
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <motion.div
          className="mb-16 text-center"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.3 }}
          variants={headerVariants}
        >
          <div
            className="inline-flex items-center gap-2 mb-4 px-3 py-1.5 rounded-full"
            style={{
              background: `linear-gradient(to right, ${currentTheme.accent.teal}20, ${currentTheme.accent.indigo}20)`,
              border: `1px solid ${currentTheme.accent.teal}66`,
            }}
          >
            <Zap size={16} style={{ color: currentTheme.accent.teal }} />
            <span className="text-sm font-semibold" style={{ color: currentTheme.accent.teal }}>
              Agentic Operator vs kagent
            </span>
          </div>
          <h2
            className="text-4xl md:text-5xl font-bold mb-4 leading-tight"
            style={{ color: currentTheme.text.primary }}
          >
            Built for teams that{' '}
            <span
              style={{
                background: `linear-gradient(to right, ${currentTheme.accent.teal}, ${currentTheme.accent.indigo})`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              cannot use the cloud
            </span>
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: currentTheme.text.tertiary }}>
            kagent is excellent for cloud-connected teams. We are the only option for environments where data cannot leave the network.
          </p>
        </motion.div>

        {/* Comparison Table */}
        <motion.div
          className="space-y-4 mb-12"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.2 }}
          variants={containerVariants}
        >
          {/* Header Row */}
          <motion.div
            className="grid grid-cols-[1fr_1fr_1fr] gap-6 mb-6 px-6 py-4 rounded-lg"
            style={{
              backgroundColor: `${currentTheme.bg.secondary}CC`,
              border: `1px solid ${currentTheme.border.light}`,
            }}
            variants={itemVariants}
          >
            <div className="font-semibold text-sm" style={{ color: currentTheme.text.tertiary }}>
              Capability
            </div>
            <div className="text-center">
              <div className="font-bold text-base" style={{ color: currentTheme.accent.teal }}>
                NineVigil
              </div>
              <div className="text-xs mt-1" style={{ color: currentTheme.text.muted }}>
                Apache 2.0 · Zero-egress
              </div>
            </div>
            <div className="text-center">
              <div className="font-bold text-base" style={{ color: currentTheme.text.tertiary }}>
                kagent (Solo.io)
              </div>
              <div className="text-xs mt-1" style={{ color: currentTheme.text.muted }}>
                CNCF Sandbox · Cloud-connected
              </div>
            </div>
          </motion.div>

          {/* Feature Rows */}
          {features.map((feature, idx) => (
            <motion.div
              key={idx}
              className="grid grid-cols-[1fr_1fr_1fr] gap-6 px-6 py-5 rounded-lg transition-colors duration-200"
              style={{
                background: `linear-gradient(to right, ${currentTheme.bg.secondary}99, ${currentTheme.bg.secondary}66)`,
                border: `1px solid ${currentTheme.border.light}`,
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = currentTheme.border.medium;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = currentTheme.border.light;
              }}
              variants={itemVariants}
            >
              {/* Feature Name */}
              <div>
                <div className="font-semibold text-sm" style={{ color: currentTheme.text.primary }}>
                  {feature.name}
                </div>
                <div className="text-xs mt-1.5" style={{ color: currentTheme.text.muted }}>
                  {feature.description}
                </div>
              </div>

              {/* Agentic Column */}
              <div className="flex items-center gap-3">
                <CheckCircle2 size={20} className="flex-shrink-0" style={{ color: currentTheme.accent.teal }} />
                <span className="text-sm" style={{ color: currentTheme.text.primary }}>
                  {feature.agentic}
                </span>
              </div>

              {/* kagent Column */}
              <div className="flex items-center gap-3">
                <XCircle size={20} className="flex-shrink-0" style={{ color: currentTheme.text.tertiary }} />
                <span className="text-sm" style={{ color: currentTheme.text.tertiary }}>
                  {feature.kagent}
                </span>
              </div>
            </motion.div>
          ))}
        </motion.div>

        {/* Context callout */}
        <motion.div
          className="mb-12 px-6 py-5 rounded-xl"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.3 }}
          variants={{
            hidden: { opacity: 0, y: 16 },
            visible: { opacity: 1, y: 0, transition: { duration: 0.6 } },
          }}
          style={{
            background: `linear-gradient(to right, ${currentTheme.accent.teal}0F, ${currentTheme.accent.indigo}0A)`,
            border: `1px solid ${currentTheme.accent.teal}33`,
          }}
        >
          <div className="flex items-start gap-3">
            <AlertTriangle size={18} className="flex-shrink-0 mt-0.5" style={{ color: currentTheme.accent.teal }} />
            <p className="text-sm leading-relaxed" style={{ color: currentTheme.text.tertiary }}>
              <span className="font-semibold" style={{ color: currentTheme.text.primary }}>kagent validates this market.</span>{' '}
              When Google, Microsoft, IBM, and Red Hat all contribute to a Kubernetes agent runtime, you don&apos;t need to convince anyone the category is real. But their enterprise revenue depends on telemetry, licensing callbacks, and managed control planes — a structural conflict with air-gapped and regulated buyers.{' '}
              <span className="font-semibold" style={{ color: currentTheme.accent.teal }}>These are different buyers. That&apos;s our market.</span>
            </p>
          </div>
        </motion.div>

        {/* CTA Section */}
        <motion.div
          className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-6"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.3 }}
          variants={{
            hidden: { opacity: 0, y: 20 },
            visible: {
              opacity: 1,
              y: 0,
              transition: { duration: 0.6, ease: 'easeOut' },
            },
          }}
        >
          <a
            href="#quickstart"
            className="px-8 py-3.5 rounded-lg font-semibold text-base transition-all duration-300 hover:scale-105 hover:brightness-110"
            style={{
              background: `linear-gradient(to right, ${currentTheme.accent.teal}, #00b88d)`,
              color: '#03231d',
            }}
          >
            Deploy in 5 Minutes
          </a>
          <a
            href="mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Inquiry"
            className="px-8 py-3.5 rounded-lg border-2 font-semibold text-base transition-all duration-300"
            style={{
              borderColor: currentTheme.border.medium,
              color: currentTheme.text.primary,
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = `${currentTheme.accent.teal}99`;
              e.currentTarget.style.color = currentTheme.accent.teal;
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = currentTheme.border.medium;
              e.currentTarget.style.color = currentTheme.text.primary;
            }}
          >
            Talk to Us
          </a>
        </motion.div>

        {/* Trust Statement */}
        <motion.div
          className="mt-12 text-center"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.3 }}
          variants={{
            hidden: { opacity: 0 },
            visible: {
              opacity: 1,
              transition: { duration: 0.8, delay: 0.3 },
            },
          }}
        >
          <p className="text-sm" style={{ color: currentTheme.text.muted }}>
            The only Kubernetes agent platform for{' '}
            <span className="font-semibold" style={{ color: currentTheme.accent.teal }}>zero-egress</span>,{' '}
            <span className="font-semibold" style={{ color: currentTheme.accent.teal }}>FedRAMP</span>, and{' '}
            <span className="font-semibold" style={{ color: currentTheme.accent.teal }}>sovereign cloud</span>{' '}
            environments
          </p>
        </motion.div>
      </div>
    </section>
  );
};

export default Comparison;
