import { motion } from 'framer-motion';
import {
  Building2,
  Users,
  Shield,
  TrendingUp,
  LineChart,
  Briefcase,
} from 'lucide-react';

const AUDIENCES = [
  {
    id: 1,
    phase: 'Product Teams',
    title: 'Competitive Feature Tracking',
    description: 'Know the moment competitors ship new features, change their UI, or update messaging. Stay ahead instead of reacting.',
    icon: Building2,
    color: '#00d4aa',
  },
  {
    id: 2,
    phase: 'Growth Teams',
    title: 'Pricing Intelligence',
    description: 'Monitor competitor pricing pages daily. Detect tier changes, new plans, and positioning shifts before they impact your pipeline.',
    icon: LineChart,
    color: '#06b6d4',
  },
  {
    id: 3,
    phase: 'Sales Teams',
    title: 'Battle Card Automation',
    description: 'Auto-generated competitive battle cards with the latest pricing, features, and positioning differences — always current, never stale.',
    icon: Shield,
    color: '#8b5cf6',
  },
  {
    id: 4,
    phase: 'Executives',
    title: 'Market Landscape Reports',
    description: 'Weekly AI-synthesized market landscape reports covering all tracked competitors. Strategic insights delivered to your inbox.',
    icon: Users,
    color: '#ec4899',
  },
  {
    id: 5,
    phase: 'VC & PE Firms',
    title: 'Portfolio Due Diligence',
    description: 'Continuously monitor portfolio companies and their competitors. Track product velocity, market positioning, and growth signals.',
    icon: Briefcase,
    color: '#f59e0b',
  },
  {
    id: 6,
    phase: 'Agencies',
    title: 'Multi-Client Intelligence',
    description: 'Manage competitive intelligence across multiple clients from one dashboard. White-label reports with your branding.',
    icon: TrendingUp,
    color: '#10b981',
  },
];

export default function Features() {
  return (
    <section className="relative py-24 px-4 sm:px-6 lg:px-8" style={{ background: '#05080f' }}>
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
              Who Uses Visual Market Intelligence (VMI)
            </h2>
            <p className="text-lg text-slate-400 max-w-2xl mx-auto">
              From product teams to VC firms — Visual Market Intelligence serves anyone who needs to know what competitors are doing.
            </p>
          </motion.div>
        </div>

        {/* Features grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {AUDIENCES.map((feature, idx) => {
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
                    <span className="text-xs text-slate-500">Early access</span>
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
            Join leading teams who use Visual Market Intelligence (VMI) to stay ahead of the competition.
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
