import React from 'react';
import { motion } from 'framer-motion';
import { CheckCircle2, Layers, Zap } from 'lucide-react';
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

  const capabilities = [
    {
      name: 'Air-Gapped Deployment',
      description: 'FedRAMP, HIPAA, sovereign cloud',
      detail: 'Fully offline. No egress, no callbacks, no cloud control plane.',
    },
    {
      name: 'Per-Workload Cost Attribution',
      description: 'Chargeback to business units',
      detail: 'Per-workload cost tracking with budget controls and tenant isolation.',
    },
    {
      name: 'DAG Workflow Orchestration',
      description: 'Multi-step agent pipelines',
      detail: 'Argo Workflows for fan-out, retries, and DAGs.',
    },
    {
      name: 'Multi-Tenant Isolation',
      description: 'Multi-tenant SaaS and regulated orgs',
      detail: 'Namespace quota plus token budget per tenant.',
    },
    {
      name: 'Open Source Core',
      description: 'Zero vendor lock-in',
      detail: 'Apache 2.0. Self-hostable and air-gapped.',
    },
  ];

  const runtimes = [
    { name: 'Clawdlinux AgentWorkload', supported: true },
    { name: 'CNCF agent runtimes (kagent, etc.)', supported: true },
    { name: 'Custom agent pods', supported: true },
    { name: 'Kubeflow pipelines', supported: true },
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
            <Layers size={16} style={{ color: currentTheme.accent.teal }} />
            <span className="text-sm font-semibold" style={{ color: currentTheme.accent.teal }}>
              Governance Plane
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
              cannot skip audit
            </span>
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: currentTheme.text.tertiary }}>
            Clawdlinux adds regulated controls around any Kubernetes agent runtime. The runtime handles lifecycle. We handle compliance.
          </p>
        </motion.div>

        {/* Capabilities */}
        <motion.div
          className="space-y-4 mb-12"
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.2 }}
          variants={containerVariants}
        >
          {capabilities.map((cap, idx) => (
            <motion.div
              key={idx}
              className="flex items-start gap-4 px-6 py-5 rounded-lg transition-colors duration-200"
              style={{
                background: `linear-gradient(to right, ${currentTheme.bg.secondary}99, ${currentTheme.bg.secondary}66)`,
                border: `1px solid ${currentTheme.border.light}`,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = currentTheme.border.medium; }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = currentTheme.border.light; }}
              variants={itemVariants}
            >
              <CheckCircle2 size={20} className="flex-shrink-0 mt-0.5" style={{ color: currentTheme.accent.teal }} />
              <div>
                <div className="font-semibold text-sm" style={{ color: currentTheme.text.primary }}>
                  {cap.name}
                </div>
                <div className="text-sm mt-1" style={{ color: currentTheme.text.secondary }}>
                  {cap.detail}
                </div>
                <div className="text-xs mt-1" style={{ color: currentTheme.text.muted }}>
                  {cap.description}
                </div>
              </div>
            </motion.div>
          ))}
        </motion.div>

        {/* Compatible runtimes */}
        <motion.div
          className="mb-12 px-6 py-6 rounded-xl"
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
          <div className="flex items-center gap-2 mb-4">
            <Zap size={16} style={{ color: currentTheme.accent.teal }} />
            <span className="font-semibold text-sm" style={{ color: currentTheme.text.primary }}>
              Compatible agent runtimes
            </span>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {runtimes.map((rt, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <CheckCircle2 size={16} style={{ color: currentTheme.accent.teal }} />
                <span className="text-sm" style={{ color: currentTheme.text.secondary }}>{rt.name}</span>
              </div>
            ))}
          </div>
          <p className="text-xs mt-4" style={{ color: currentTheme.text.muted }}>
            The runtime handles agent lifecycle and tools. Clawdlinux handles multi-tenancy, audit, spend, and air-gapped compliance. Use both.
          </p>
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
