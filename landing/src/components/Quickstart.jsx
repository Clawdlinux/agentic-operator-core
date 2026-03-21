import { motion } from 'framer-motion';
import {
  Terminal,
  Package,
  Settings2,
  Rocket,
  CheckCircle2,
  ArrowRight,
  BookOpen,
  Clock,
} from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const STEPS = [
  {
    step: '01',
    icon: Terminal,
    title: 'Clone the Repo',
    description: 'Pull the operator source in one command — no sign-up, no SaaS account required.',
    code: 'git clone https://github.com/Clawdlinux/agentic-operator-core',
    accentColor: '#00d4aa',
  },
  {
    step: '02',
    icon: Package,
    title: 'Install via Helm',
    description: 'Deploy the operator to any running Kubernetes cluster with a single Helm command.',
    code: 'helm install agentic-op ./charts/agentic-operator',
    accentColor: '#6366f1',
  },
  {
    step: '03',
    icon: Settings2,
    title: 'Configure a Workload',
    description: 'Apply a sample AgentWorkload CRD to set namespace, model, quota, and egress policy.',
    code: 'kubectl apply -f config/samples/agentworkload.yaml',
    accentColor: '#00d4aa',
  },
  {
    step: '04',
    icon: Rocket,
    title: 'Deploy the Agent',
    description: 'The operator schedules your agent pod with full policy isolation and cost attribution.',
    code: 'kubectl apply -f my-agent-workload.yaml',
    accentColor: '#6366f1',
  },
  {
    step: '05',
    icon: CheckCircle2,
    title: 'Watch It Run',
    description: 'Monitor agent status, cost metrics, and execution logs in real time from kubectl.',
    code: 'kubectl get agentworkloads -n agentic-system -w',
    accentColor: '#00d4aa',
  },
];

const containerVariants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.1, delayChildren: 0.05 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 28 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.55, ease: 'easeOut' } },
};

function StepCard({ step, currentTheme }) {
  const Icon = step.icon;
  return (
    <motion.div
      variants={itemVariants}
      className="relative rounded-xl p-5 transition-all duration-300 group flex flex-col"
      style={{
        background: `${currentTheme.bg.secondary}CC`,
        border: `1px solid ${currentTheme.border.light}`,
      }}
    >
      <div
        className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
        style={{
          background: `radial-gradient(circle at 50% 0%, ${step.accentColor}12 0%, transparent 70%)`,
        }}
      />
      <div className="relative z-10 flex flex-col flex-1">
        {/* Step number + icon row */}
        <div className="flex items-center gap-3 mb-3">
          <span
            className="text-3xl font-bold leading-none tabular-nums select-none"
            style={{
              fontFamily: "'IBM Plex Mono', monospace",
              color: `${step.accentColor}35`,
            }}
          >
            {step.step}
          </span>
          <div
            className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 group-hover:scale-105 transition-transform duration-200"
            style={{
              background: `${step.accentColor}14`,
              border: `1px solid ${step.accentColor}28`,
            }}
          >
            <Icon size={15} style={{ color: step.accentColor }} strokeWidth={1.75} />
          </div>
        </div>

        <h4
          className="text-sm font-semibold mb-1.5"
          style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
        >
          {step.title}
        </h4>

        <p
          className="text-xs leading-relaxed flex-1 mb-3"
          style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.tertiary }}
        >
          {step.description}
        </p>

        {/* Code snippet */}
        <div
          className="rounded-md p-2.5"
          style={{
            background: `${currentTheme.bg.primary}A6`,
            border: `1px solid ${currentTheme.border.light}`,
          }}
        >
          <code
            className="text-[10.5px] leading-snug break-all"
            style={{ fontFamily: "'IBM Plex Mono', monospace", color: step.accentColor }}
          >
            {step.code}
          </code>
        </div>
      </div>
    </motion.div>
  );
}

export default function Quickstart() {
  const { currentTheme } = useTheme();

  return (
    <section
      id="quickstart"
      className="relative py-24 px-4 sm:px-6 lg:px-8 overflow-hidden"
      style={{
        background: currentTheme.bg.primary,
        transition: 'background-color 300ms ease-in-out',
      }}
    >
      {/* Background glow */}
      <div
        className="absolute pointer-events-none"
        style={{
          top: '30%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: 900,
          height: 400,
          borderRadius: '50%',
          background:
            `radial-gradient(circle, ${currentTheme.accent.indigo}12 0%, ${currentTheme.accent.teal}0A 50%, transparent 70%)`,
          filter: 'blur(70px)',
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
              background: `${currentTheme.accent.indigo}14`,
              border: `1px solid ${currentTheme.accent.indigo}40`,
              color: currentTheme.accent.indigo,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            <Clock size={13} />
            5-Minute Quickstart
          </div>

          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold leading-tight mb-4"
            style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
          >
            From Repo to{' '}
            <span
              style={{
                background: `linear-gradient(135deg, ${currentTheme.accent.teal}, ${currentTheme.accent.indigo})`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Running Agent
            </span>
          </h2>

          <p
            className="text-base sm:text-lg max-w-2xl mx-auto"
            style={{ fontFamily: "'DM Sans', sans-serif", color: currentTheme.text.tertiary }}
          >
            No SaaS account. No vendor lock-in. Five commands and your first agent is live on
            Kubernetes.
          </p>
        </motion.div>

        {/* Steps */}
        <div className="relative">
          {/* Desktop connector line */}
          <div
            className="hidden lg:block absolute pointer-events-none"
            style={{
              top: 44,
              left: '9%',
              right: '9%',
              height: 1,
              background:
                `linear-gradient(to right, transparent, ${currentTheme.accent.teal}33 20%, ${currentTheme.accent.indigo}33 80%, transparent)`,
            }}
          />

          <motion.div
            variants={containerVariants}
            initial="hidden"
            whileInView="visible"
            viewport={{ once: true, margin: '-60px' }}
            className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4"
          >
            {STEPS.map((step) => (
              <StepCard key={step.step} step={step} currentTheme={currentTheme} />
            ))}
          </motion.div>
        </div>

        {/* CTA block */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: '-60px' }}
          transition={{ duration: 0.6, delay: 0.3, ease: 'easeOut' }}
          className="mt-14 flex flex-col sm:flex-row items-center justify-center gap-4"
        >
          <a
            href="https://github.com/Clawdlinux/agentic-operator-core/blob/main/docs/01-quickstart.md"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-7 py-3.5 rounded-xl text-sm font-semibold transition-all duration-200 hover:brightness-110 hover:shadow-xl active:scale-[0.97]"
            style={{
              background: `linear-gradient(135deg, ${currentTheme.accent.teal} 0%, #00b894 100%)`,
              color: '#03231d',
            }}
          >
            <BookOpen size={16} />
            Read Full Quickstart
          </a>

          <a
            href="https://github.com/Clawdlinux/agentic-operator-core"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-7 py-3.5 rounded-xl text-sm font-semibold transition-all duration-200 active:scale-[0.97]"
            style={{
              border: `1px solid ${currentTheme.border.medium}`,
              color: currentTheme.text.primary,
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = `${currentTheme.accent.teal}80`;
              e.currentTarget.style.color = currentTheme.accent.teal;
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = currentTheme.border.medium;
              e.currentTarget.style.color = currentTheme.text.primary;
            }}
          >
            Star on GitHub
            <ArrowRight size={16} />
          </a>
        </motion.div>
      </div>
    </section>
  );
}
