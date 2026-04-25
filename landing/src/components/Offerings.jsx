import { motion } from "framer-motion";
import {
  Shield,
  Network,
  Cpu,
  Lock,
  GitBranch,
  Layers,
  WifiOff,
  Receipt,
} from "lucide-react";
import { useTheme } from "../hooks/useTheme";

const features = [
  {
    icon: Shield,
    title: "Policy-Aware Isolation",
    description:
      "Every AgentWorkload gets namespace, identity, and storage boundaries without hand-written cluster glue.",
  },
  {
    icon: Network,
    title: "Cilium FQDN Egress",
    description:
      "Restrict outbound agent traffic to approved destinations and keep runtime access inside policy guardrails.",
  },
  {
    icon: Cpu,
    title: "Pluggable Agent Runtimes",
    description:
      "Run browser automation, LLM workers, or custom containers through the same operator reconciliation flow.",
  },
  {
    icon: Lock,
    title: "Scoped Secrets & Identity",
    description:
      "Bind secrets, service accounts, and access controls to the workload instead of sharing cluster-wide credentials.",
  },
  {
    icon: GitBranch,
    title: "Argo DAG Orchestration",
    description:
      "Translate multi-step agent jobs into Argo Workflows with retries, status transitions, and step-level visibility.",
  },
  {
    icon: Layers,
    title: "Artifacts & State Layers",
    description:
      "Persist prompts, outputs, and workflow artifacts to MinIO so operators can inspect every run after completion.",
  },
  {
    icon: WifiOff,
    title: "Air-Gapped Deployments",
    description:
      "Run in fully offline environments with no cloud egress, no licensing callbacks, and no telemetry — built for FedRAMP, HIPAA, and sovereign cloud constraints.",
  },
  {
    icon: Receipt,
    title: "Per-Workload Cost Tracking",
    description:
      "Track token usage per workload and per tenant, enabling precise cost attribution and chargeback to business units without manual tracking.",
  },
];

const containerVariants = {
  hidden: {},
  visible: {
    transition: {
      staggerChildren: 0.1,
    },
  },
};

const cardVariants = {
  hidden: { opacity: 0, y: 32 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: "easeOut" },
  },
};

const withAlpha = (hex, alpha) => `${hex}${alpha}`;

function FeatureCard({ feature, currentTheme, theme }) {
  const Icon = feature.icon;

  return (
    <motion.div
      variants={cardVariants}
      whileHover={{
        borderColor: withAlpha(currentTheme.accent.teal, "73"),
        boxShadow:
          theme === "dark"
            ? `0 0 28px ${withAlpha(currentTheme.accent.teal, "1F")}, 0 4px 24px rgba(0,0,0,0.4)`
            : `0 0 22px ${withAlpha(currentTheme.accent.teal, "14")}, 0 4px 18px rgba(15,23,42,0.12)`,
        y: -4,
      }}
      className="rounded-xl p-6 flex flex-col gap-4 cursor-default transition-colors duration-300"
      style={{
        background:
          theme === "dark"
            ? withAlpha(currentTheme.bg.secondary, "B3")
            : withAlpha(currentTheme.bg.secondary, "E6"),
        border: `1px solid ${currentTheme.border.light}`,
        backdropFilter: "blur(12px)",
      }}
    >
      <div
        className="w-11 h-11 rounded-xl flex items-center justify-center flex-shrink-0"
        style={{
          background: withAlpha(currentTheme.accent.teal, theme === "dark" ? "1F" : "14"),
          border: `1px solid ${withAlpha(currentTheme.accent.teal, "40")}`,
        }}
      >
        <Icon size={22} color={currentTheme.accent.teal} strokeWidth={1.75} />
      </div>

      <div>
        <h3
          className="text-base font-semibold mb-2"
          style={{
            fontFamily: "'Syne', sans-serif",
            color: currentTheme.text.primary,
          }}
        >
          {feature.title}
        </h3>
        <p
          className="text-sm leading-relaxed"
          style={{
            fontFamily: "'DM Sans', sans-serif",
            color: currentTheme.text.tertiary,
          }}
        >
          {feature.description}
        </p>
      </div>
    </motion.div>
  );
}

export default function Offerings() {
  const { currentTheme, theme } = useTheme();

  return (
    <section
      id="features"
      className="py-24 px-4 transition-colors duration-300"
      style={{ background: currentTheme.bg.primary }}
    >
      <div className="max-w-6xl mx-auto">
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="text-center mb-16"
        >
          <div
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-semibold uppercase tracking-widest mb-6"
            style={{
              background: withAlpha(currentTheme.accent.teal, theme === "dark" ? "14" : "10"),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, "40")}`,
              color: theme === "dark" ? currentTheme.accent.teal : '#1e40af',
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            Operator Capabilities
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold leading-tight"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: currentTheme.text.primary,
            }}
          >
            Everything You Need for{" "}
            <span
              style={{
                background: `linear-gradient(135deg, ${currentTheme.accent.teal}, ${currentTheme.accent.indigo})`,
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
              }}
            >
              Zero-Egress Agent Workloads
            </span>
          </h2>
        </motion.div>

        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, margin: "-60px" }}
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5"
        >
          {features.map((feature) => (
            <FeatureCard
              key={feature.title}
              feature={feature}
              currentTheme={currentTheme}
              theme={theme}
            />
          ))}
        </motion.div>
      </div>
    </section>
  );
}
