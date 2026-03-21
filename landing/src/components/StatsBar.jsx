import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { useTheme } from "../hooks/useTheme";

const stats = [
  {
    value: 1,
    prefix: "v0.",
    suffix: ".0",
    label: "Latest Release",
    sublabel: "stable tag",
    color: "#00d4aa",
    colorClass: "text-teal-400",
  },
  {
    value: 25,
    prefix: "K8s 1.",
    suffix: "+",
    label: "Kubernetes",
    sublabel: "minimum cluster version",
    color: "#6366f1",
    colorClass: "text-violet-400",
  },
  {
    value: 22,
    prefix: "Go 1.",
    suffix: "",
    label: "Go Runtime",
    sublabel: "required build version",
    color: "#f59e0b",
    colorClass: "text-amber-400",
  },
  {
    value: 2,
    prefix: "Apache ",
    suffix: ".0",
    label: "License",
    sublabel: "open source",
    color: "#22c55e",
    colorClass: "text-green-400",
  },
];

function useCountUp(target, duration = 1800, active = false) {
  const [count, setCount] = useState(0);

  useEffect(() => {
    if (!active) return;
    let startTime = null;
    const step = (timestamp) => {
      if (!startTime) startTime = timestamp;
      const progress = Math.min((timestamp - startTime) / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      setCount(Math.floor(eased * target));
      if (progress < 1) requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  }, [target, duration, active]);

  return count;
}

function StatItem({ stat, active, index, currentTheme }) {
  const count = useCountUp(stat.value, 1600, active);
  const isLast = index === stats.length - 1;

  return (
    <div className="flex items-stretch flex-1 min-w-0">
      <div className="flex flex-col items-center justify-center px-6 py-8 flex-1 group">
        <div
          className="font-bold leading-none mb-2 tracking-tight transition-all duration-300 group-hover:scale-105"
          style={{
            fontFamily: "'Syne', sans-serif",
            fontSize: "clamp(2rem, 4vw, 3rem)",
            color: stat.color,
            textShadow: `0 0 32px ${stat.color}55`,
          }}
        >
          {stat.prefix}
          {count}
          {stat.suffix}
        </div>
        <div
          className="text-sm font-semibold uppercase tracking-widest mb-0.5"
          style={{
            fontFamily: "'DM Sans', sans-serif",
            color: stat.color,
            opacity: 0.85,
          }}
        >
          {stat.label}
        </div>
        <div
          className="text-xs"
          style={{
            fontFamily: "'DM Sans', sans-serif",
            color: currentTheme.text.tertiary,
          }}
        >
          {stat.sublabel}
        </div>
      </div>
      {!isLast && (
        <div
          className="w-px self-stretch my-4"
          style={{
            background:
              `linear-gradient(to bottom, transparent, ${currentTheme.border.light}, transparent)`,
          }}
        />
      )}
    </div>
  );
}

export default function StatsBar() {
  const { currentTheme } = useTheme();
  const isInView = true;

  return (
    <motion.section
      initial={{ opacity: 0, y: 24 }}
      animate={isInView ? { opacity: 1, y: 0 } : {}}
      transition={{ duration: 0.6, ease: "easeOut" }}
      style={{
        background: currentTheme.bg.overlay,
        borderTop: `1px solid ${currentTheme.border.light}`,
        borderBottom: `1px solid ${currentTheme.border.light}`,
      }}
    >
      <div className="max-w-6xl mx-auto">
        <div className="flex flex-col sm:flex-row divide-y sm:divide-y-0">
          {stats.map((stat, i) => (
            <StatItem
              key={stat.label}
              stat={stat}
              active={isInView}
              index={i}
              currentTheme={currentTheme}
            />
          ))}
        </div>
      </div>
    </motion.section>
  );
}
