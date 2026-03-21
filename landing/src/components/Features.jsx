import { motion } from 'framer-motion';
import {
  Cpu,
  AlertTriangle,
  Shield,
  GitMerge,
  Layers,
  GitPullRequest,
} from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const AUDIENCES = [
  {
    id: 1,
    phase: 'Platform Teams',
    title: 'Namespace Automation',
    description: 'Provision isolated agent namespaces, service accounts, storage, and policy from one declarative workload spec.',
    icon: Cpu,
    color: '#00d4aa',
  },
  {
    id: 2,
    phase: 'SREs',
    title: 'Operational Guardrails',
    description: 'Bake retries, quotas, and workflow lifecycle controls into every run instead of patching them together per team.',
    icon: AlertTriangle,
    color: '#06b6d4',
  },
  {
    id: 3,
    phase: 'Security Teams',
    title: 'Policy-Aware Egress',
    description: 'Restrict outbound traffic with Cilium FQDN policies and keep agent traffic inside approved destinations.',
    icon: Shield,
    color: '#8b5cf6',
  },
  {
    id: 4,
    phase: 'DevOps',
    title: 'Workflow Orchestration',
    description: 'Map multi-step agent jobs to Argo DAGs with retries, status visibility, and artifact handoff out of the box.',
    icon: GitMerge,
    color: '#ec4899',
  },
  {
    id: 5,
    phase: 'Multi-Tenant SaaS',
    title: 'Tenant Isolation',
    description: 'Run many customer workloads per cluster without credential bleed, namespace collisions, or noisy-neighbor drift.',
    icon: Layers,
    color: '#f59e0b',
  },
  {
    id: 6,
    phase: 'OSS Contributors',
    title: 'Extensible Control Plane',
    description: 'Extend CRDs, controllers, and worker images to fit your own agent runtimes, network posture, and artifact model.',
    icon: GitPullRequest,
    color: '#10b981',
  },
];

const withAlpha = (hex, alpha) => `${hex}${alpha}`;

export default function Features() {
  const { currentTheme, theme } = useTheme();

  return (
    <section
      className="relative py-24 px-4 sm:px-6 lg:px-8 transition-colors duration-300"
      style={{ background: currentTheme.bg.primary }}
    >
      {/* Background gradient */}
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          background: `radial-gradient(circle at 50% 0%, ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10')} 0%, rgba(0,212,170,0) 50%)`,
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
              className="text-4xl sm:text-5xl font-bold mb-4"
              style={{
                fontFamily: "'Syne', sans-serif",
                color: currentTheme.text.primary,
              }}
            >
              Who Deploys Clawdlinux
            </h2>
            <p className="text-lg max-w-2xl mx-auto" style={{ color: currentTheme.text.tertiary }}>
              From platform engineering to security review, Clawdlinux fits teams standardizing how autonomous agents run on Kubernetes.
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
                className="group relative p-6 rounded-xl transition-all duration-300"
                style={{
                  border: `1px solid ${currentTheme.border.light}`,
                  background: `linear-gradient(to bottom, ${withAlpha(currentTheme.bg.secondary, theme === 'dark' ? '6E' : 'CC')}, transparent)`,
                }}
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
                  <h3 className="text-lg font-bold mb-2" style={{ color: currentTheme.text.primary }}>
                    {feature.title}
                  </h3>

                  {/* Description */}
                  <p className="text-sm" style={{ color: currentTheme.text.tertiary }}>
                    {feature.description}
                  </p>

                  {/* Checkmark footer */}
                  <div
                    className="mt-4 pt-4 flex items-center gap-2"
                    style={{ borderTop: `1px solid ${currentTheme.border.light}` }}
                  >
                    <div className="w-1.5 h-1.5 rounded-full" style={{ background: feature.color }} />
                    <span className="text-xs" style={{ color: currentTheme.text.muted }}>
                      Open source
                    </span>
                  </div>
                </div>
              </motion.div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
