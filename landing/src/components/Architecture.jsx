import { useRef } from "react";
import { motion, useInView } from "framer-motion";
import { Target, Camera, Sparkles, Send } from "lucide-react";

const steps = [
  {
    number: "01",
    icon: Target,
    title: "Configure Your Targets",
    description:
      "Define which competitor pages to monitor — pricing, product, about, changelog. Set schedules and alert preferences.",
    details: ["Point-and-click target setup", "Custom monitoring schedules", "Multi-region capture support", "YAML or dashboard configuration"],
    color: "#00d4aa",
    colorAlpha: "rgba(0, 212, 170, 0.12)",
    colorBorder: "rgba(0, 212, 170, 0.25)",
  },
  {
    number: "02",
    icon: Camera,
    title: "Automated Screenshot Capture",
    description:
      "VMI captures pixel-perfect screenshots of every target page on your schedule — daily, hourly, or custom intervals.",
    details: ["Full-page & above-fold capture", "Mobile and desktop viewports", "JavaScript-rendered pages supported", "Geo-distributed capture nodes"],
    color: "#6366f1",
    colorAlpha: "rgba(99, 102, 241, 0.12)",
    colorBorder: "rgba(99, 102, 241, 0.25)",
  },
  {
    number: "03",
    icon: Sparkles,
    title: "AI-Powered Change Detection",
    description:
      "Our AI compares screenshots against baselines, identifies visual changes, and analyzes what they mean — pricing updates, new features, messaging shifts.",
    details: ["Visual diff with pixel precision", "AI classifies change type & severity", "Natural language change summaries", "Historical trend analysis"],
    color: "#f59e0b",
    colorAlpha: "rgba(245, 158, 11, 0.12)",
    colorBorder: "rgba(245, 158, 11, 0.25)",
  },
  {
    number: "04",
    icon: Send,
    title: "Insights Delivered to You",
    description:
      "Receive structured reports via Slack, email, or dashboard — with AI analysis explaining what changed and recommended actions.",
    details: ["PDF and dashboard reports", "Slack and email delivery", "AI-generated action recommendations", "Team sharing & collaboration"],
    color: "#22c55e",
    colorAlpha: "rgba(34, 197, 94, 0.12)",
    colorBorder: "rgba(34, 197, 94, 0.25)",
  },
];

function GridPattern() {
  return (
    <svg
      className="absolute inset-0 w-full h-full"
      style={{ opacity: 0.025 }}
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <pattern
          id="arch-grid"
          width="40"
          height="40"
          patternUnits="userSpaceOnUse"
        >
          <path
            d="M 40 0 L 0 0 0 40"
            fill="none"
            stroke="white"
            strokeWidth="1"
          />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#arch-grid)" />
    </svg>
  );
}

function ConnectorArrow({ color }) {
  return (
    <div className="hidden lg:flex items-center justify-center w-16 flex-shrink-0 relative">
      <motion.div
        initial={{ scaleX: 0, opacity: 0 }}
        whileInView={{ scaleX: 1, opacity: 1 }}
        viewport={{ once: true }}
        transition={{ duration: 0.6, ease: "easeOut", delay: 0.4 }}
        className="w-full h-0.5 relative"
        style={{
          background: `linear-gradient(90deg, ${color}40, ${color}90)`,
          transformOrigin: "left center",
        }}
      >
        <div
          className="absolute right-0 top-1/2 -translate-y-1/2"
          style={{
            width: 0,
            height: 0,
            borderTop: "5px solid transparent",
            borderBottom: "5px solid transparent",
            borderLeft: `7px solid ${color}90`,
          }}
        />
      </motion.div>
    </div>
  );
}

function StepCard({ step, index }) {
  const Icon = step.icon;

  return (
    <motion.div
      initial={{ opacity: 0, y: 32 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ duration: 0.55, ease: "easeOut", delay: index * 0.2 }}
      className="flex-1 min-w-0 flex flex-col"
    >
      <div
        className="rounded-2xl p-6 flex flex-col gap-5 h-full relative overflow-hidden group"
        style={{
          background: "rgba(13, 21, 37, 0.8)",
          border: `1px solid ${step.colorBorder}`,
          backdropFilter: "blur(12px)",
        }}
      >
        {/* Glow effect on hover */}
        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none"
          style={{
            background: `radial-gradient(ellipse at top left, ${step.colorAlpha} 0%, transparent 70%)`,
          }}
        />

        {/* Number badge */}
        <div className="flex items-center justify-between">
          <div
            className="text-xs font-bold tracking-widest"
            style={{
              fontFamily: "'IBM Plex Mono', monospace",
              color: step.color,
              opacity: 0.6,
            }}
          >
            {step.number}
          </div>
          <div
            className="w-10 h-10 rounded-xl flex items-center justify-center"
            style={{
              background: step.colorAlpha,
              border: `1px solid ${step.colorBorder}`,
            }}
          >
            <Icon size={20} color={step.color} strokeWidth={1.75} />
          </div>
        </div>

        <div>
          <h3
            className="text-base font-bold mb-2"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: "#e2e8f0",
            }}
          >
            {step.title}
          </h3>
          <p
            className="text-sm leading-relaxed"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              color: "#94a3b8",
            }}
          >
            {step.description}
          </p>
        </div>

        {/* Details list */}
        <ul className="mt-auto space-y-1.5">
          {step.details.map((detail, i) => (
            <li key={i} className="flex items-start gap-2 text-xs" style={{ color: "#94a3b8", fontFamily: "'DM Sans', sans-serif" }}>
              <span className="mt-0.5 block w-1 h-1 rounded-full flex-shrink-0" style={{ background: step.color }} />
              {detail}
            </li>
          ))}
        </ul>
      </div>
    </motion.div>
  );
}

export default function Architecture() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: "-80px" });

  return (
    <section
      id="architecture"
      ref={ref}
      className="py-24 px-4 relative overflow-hidden"
      style={{ background: "#05080f" }}
    >
      <GridPattern />

      {/* Ambient glow */}
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-2/3 h-64 pointer-events-none"
        style={{
          background:
            "radial-gradient(ellipse, rgba(0, 212, 170, 0.04) 0%, transparent 70%)",
        }}
      />

      <div className="max-w-6xl mx-auto relative z-10">
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
              background: "rgba(0, 212, 170, 0.08)",
              border: "1px solid rgba(0, 212, 170, 0.2)",
              color: "#00d4aa",
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            How It Works
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: "#e2e8f0",
            }}
          >
            From Target to{" "}
            <span
              style={{
                background: "linear-gradient(135deg, #00d4aa, #6366f1)",
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
              }}
            >
              Insight
            </span>
            {" "}in Minutes
          </h2>
          <p
            className="mt-4 text-base max-w-xl mx-auto"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              color: "#94a3b8",
            }}
          >
            Four automated steps turn competitor pages into actionable intelligence — no manual work required.
          </p>
        </motion.div>

        {/* Steps — desktop: horizontal row with arrows, mobile: vertical stack */}
        <div className="flex flex-col lg:flex-row items-stretch gap-4 lg:gap-0">
          {steps.map((step, index) => (
            <div key={step.number} className="flex flex-col lg:flex-row items-stretch flex-1 min-w-0">
              <StepCard step={step} index={index} />
              {index < steps.length - 1 && (
                <ConnectorArrow color={step.color} />
              )}
            </div>
          ))}
        </div>

        {/* Mobile: vertical connector lines */}
        <div className="flex lg:hidden flex-col items-center mt-0">
          {steps.slice(0, -1).map((step, i) => (
            <motion.div
              key={i}
              initial={{ scaleY: 0, opacity: 0 }}
              whileInView={{ scaleY: 1, opacity: 1 }}
              viewport={{ once: true }}
              transition={{ duration: 0.4, delay: i * 0.15 }}
              className="w-0.5 h-6 my-1"
              style={{
                background: `linear-gradient(to bottom, ${step.color}60, ${steps[i + 1].color}60)`,
                transformOrigin: "top center",
              }}
            />
          ))}
        </div>

        {/* Bottom CTA */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6, delay: 0.8 }}
          className="text-center mt-16"
        >
          <div
            className="inline-flex items-center gap-3 px-6 py-4 rounded-xl"
            style={{
              background: "rgba(13, 21, 37, 0.8)",
              border: "1px solid rgba(255,255,255,0.08)",
            }}
          >
            <span
              className="text-sm"
              style={{
                fontFamily: "'IBM Plex Mono', monospace",
                color: "#94a3b8",
              }}
            >
              Total time from target to insight:
            </span>
            <span
              className="text-sm font-bold"
              style={{
                fontFamily: "'IBM Plex Mono', monospace",
                color: "#00d4aa",
              }}
            >
              under 5 minutes
            </span>
          </div>
        </motion.div>
      </div>
    </section>
  );
}
