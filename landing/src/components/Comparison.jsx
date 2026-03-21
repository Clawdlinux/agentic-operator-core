import React from 'react';
import { motion } from 'framer-motion';
import { CheckCircle2, XCircle, Zap } from 'lucide-react';
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
      name: 'Activation Speed',
      agentic: '5-min quickstart',
      orkes: '15+ min setup',
      agenticWins: true,
      description: 'Time to first agent',
    },
    {
      name: 'Cost Transparency',
      agentic: 'Cost-aware routing + quotas',
      orkes: 'Opaque usage meters',
      agenticWins: true,
      description: 'Budget control',
    },
    {
      name: 'Deployment Model',
      agentic: 'Pure open-source self-managed',
      orkes: 'SaaS-first proprietary',
      agenticWins: true,
      description: 'Full control & ownership',
    },
    {
      name: 'Agent Control',
      agentic: 'Full policy isolation + DAG design',
      orkes: 'Limited workflow controls',
      agenticWins: true,
      description: 'Advanced orchestration',
    },
    {
      name: 'Enterprise Support',
      agentic: 'Open community + enterprise package',
      orkes: 'SaaS lock-in model',
      agenticWins: true,
      description: 'Flexibility & opt-in support',
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
              Competitive Advantage
            </span>
          </div>
          <h2
            className="text-4xl md:text-5xl font-bold mb-4 leading-tight"
            style={{ color: currentTheme.text.primary }}
          >
            Why enterprises choose{' '}
            <span
              style={{
                background: `linear-gradient(to right, ${currentTheme.accent.teal}, ${currentTheme.accent.indigo})`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Agentic Operator
            </span>
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: currentTheme.text.tertiary }}>
            Open-source flexibility with enterprise-grade control. No lock-in, no surprises.
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
              Feature
            </div>
            <div className="text-center">
              <div className="font-bold text-base" style={{ color: currentTheme.accent.teal }}>
                Agentic Operator
              </div>
              <div className="text-xs mt-1" style={{ color: currentTheme.text.muted }}>
                Open Source
              </div>
            </div>
            <div className="text-center">
              <div className="font-bold text-base" style={{ color: currentTheme.text.tertiary }}>
                Orkes
              </div>
              <div className="text-xs mt-1" style={{ color: currentTheme.text.muted }}>
                Proprietary
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

              {/* Orkes Column */}
              <div className="flex items-center gap-3">
                <XCircle size={20} className="flex-shrink-0" style={{ color: currentTheme.text.tertiary }} />
                <span className="text-sm" style={{ color: currentTheme.text.tertiary }}>
                  {feature.orkes}
                </span>
              </div>
            </motion.div>
          ))}
        </motion.div>

        {/* CTA Section */}
        <motion.div
          className="mt-16 flex flex-col sm:flex-row items-center justify-center gap-6"
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
            Start in 5 Minutes
          </a>
          <a
            href="#"
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
            View Full Comparison
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
            Join teams running <span className="font-semibold" style={{ color: currentTheme.accent.teal }}>1000+</span>{' '}
            agentic workflows in production
          </p>
        </motion.div>
      </div>
    </section>
  );
};

export default Comparison;
