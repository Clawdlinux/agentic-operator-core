import { motion } from 'framer-motion';
import {
  Zap,
  Cpu,
  Shield,
  TrendingUp,
  Lock,
  AlertCircle,
  BarChart3,
  Users,
} from 'lucide-react';

const PHASES = [
  {
    id: 1,
    phase: 'Phase 3',
    title: 'Cost-Aware Model Routing',
    description: 'Automatically select models based on task complexity. Validation → cheap, analysis → medium, reasoning → expensive. 70-80% cost reduction.',
    icon: Zap,
    color: '#00d4aa',
  },
  {
    id: 2,
    phase: 'Phase 4',
    title: 'Agent Quality Evaluation',
    description: 'Real-time quality scoring (0-100) of every LLM response. Measures relevance, hallucination risk, completeness, clarity.',
    icon: BarChart3,
    color: '#06b6d4',
  },
  {
    id: 3,
    phase: 'Phase 5',
    title: 'Production Hardening',
    description: 'Exponential backoff retry, circuit breaker for providers, automatic PII/secret scrubbing in logs.',
    icon: Shield,
    color: '#8b5cf6',
  },
  {
    id: 4,
    phase: 'Phase 6',
    title: 'Customer Onboarding',
    description: '5-minute quick-start guide. Helm charts, examples for 3 provider patterns. Production-ready for day 1.',
    icon: Users,
    color: '#ec4899',
  },
  {
    id: 5,
    phase: 'Phase 7',
    title: 'Multi-Tenant + SLA',
    description: 'Namespace isolation, per-tenant quotas (daily limits), cost budgets, SLA success rate tracking with breach detection.',
    icon: Lock,
    color: '#f59e0b',
  },
  {
    id: 6,
    phase: 'Phase 8',
    title: 'Auto-Scaling by SLA',
    description: 'Dynamic scaling based on success rate. Critical (<50%) → +2 replicas + model downgrade. Healthy (>95%) → -1 replica.',
    icon: TrendingUp,
    color: '#10b981',
  },
];

export default function Features() {
  return (
    <section id="features" className="relative py-24 px-4 sm:px-6 lg:px-8" style={{ background: '#05080f' }}>
      {/* Background gradient */}
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          background:
            'radial-gradient(circle at 50% 0%, rgba(0,212,170,0.08) 0%, rgba(0,212,170,0) 50%)',
        }}
      />

      <div className="relative z-10 max-w-7xl mx-auto">
        {/* Section header */}
        <div className="text-center mb-16">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6 }}
            viewport={{ once: true }}
          >
            <h2
              className="text-4xl sm:text-5xl font-bold mb-4 text-white"
              style={{ fontFamily: "'Syne', sans-serif" }}
            >
              Enterprise Features
            </h2>
            <p className="text-lg text-slate-400 max-w-2xl mx-auto">
              6 production-ready phases delivering complete autonomous agent infrastructure: from cost optimization to multi-tenant SLA enforcement.
            </p>
          </motion.div>
        </div>

        {/* Features grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {PHASES.map((feature, idx) => {
            const Icon = feature.icon;
            return (
              <motion.div
                key={feature.id}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, delay: idx * 0.1 }}
                viewport={{ once: true }}
                className="group relative p-6 rounded-xl border border-white/8 bg-gradient-to-b from-white/5 to-transparent hover:border-white/15 transition-all duration-300"
              >
                {/* Hover glow effect */}
                <div
                  className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
                  style={{
                    background: `radial-gradient(circle at 50% 0%, ${feature.color}15 0%, transparent 70%)`,
                  }}
                />

                {/* Content */}
                <div className="relative z-10">
                  {/* Icon */}
                  <div className="mb-4 inline-flex p-3 rounded-lg" style={{ background: `${feature.color}15` }}>
                    <Icon className="w-6 h-6" style={{ color: feature.color }} />
                  </div>

                  {/* Phase label */}
                  <div className="mb-2 inline-block text-xs font-semibold px-2 py-1 rounded-full" style={{ background: `${feature.color}20`, color: feature.color }}>
                    {feature.phase}
                  </div>

                  {/* Title */}
                  <h3 className="text-lg font-bold text-white mb-2">{feature.title}</h3>

                  {/* Description */}
                  <p className="text-sm text-slate-400">{feature.description}</p>

                  {/* Checkmark footer */}
                  <div className="mt-4 pt-4 border-t border-white/5 flex items-center gap-2">
                    <div className="w-1.5 h-1.5 rounded-full" style={{ background: feature.color }} />
                    <span className="text-xs text-slate-500">Production ready</span>
                  </div>
                </div>
              </motion.div>
            );
          })}
        </div>

        {/* CTA section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.6 }}
          viewport={{ once: true }}
          className="mt-16 text-center"
        >
          <p className="text-slate-400 mb-6">
            All phases tested with 77/77 unit + integration tests passing. Ready for enterprise deployment.
          </p>
          <a
            href="#waitlist"
            onClick={(e) => {
              e.preventDefault();
              document.querySelector('#waitlist')?.scrollIntoView({ behavior: 'smooth' });
            }}
            className="inline-flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl transition-all duration-200 hover:brightness-110 hover:shadow-xl active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
              color: '#05080f',
            }}
          >
            Get Started Today
          </a>
        </motion.div>
      </div>
    </section>
  );
}
