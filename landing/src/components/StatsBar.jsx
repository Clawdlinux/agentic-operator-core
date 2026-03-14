import { useEffect, useRef, useState } from "react";
import { motion, useInView } from "framer-motion";

const stats = [
  {
    value: 50,
    prefix: "",
    suffix: "+",
    label: "Pages Monitored",
    sublabel: "per customer",
    color: "#00d4aa",
    colorClass: "text-teal-400",
  },
  {
    value: 5,
    prefix: "<",
    suffix: " min",
    label: "Report Delivery",
    sublabel: "scan to insight",
    color: "#6366f1",
    colorClass: "text-violet-400",
  },
  {
    value: 8,
    prefix: "",
    suffix: " hrs",
    label: "Time Saved",
    sublabel: "per analysis cycle",
    color: "#f59e0b",
    colorClass: "text-amber-400",
  },
  {
    value: 320,
    prefix: "$",
    suffix: "+",
    label: "Cost Saved",
    sublabel: "per report vs manual",
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

function StatItem({ stat, active, index }) {
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
            color: "#94a3b8",
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
              "linear-gradient(to bottom, transparent, rgba(255,255,255,0.08), transparent)",
          }}
        />
      )}
    </div>
  );
}

export default function StatsBar() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: "-80px" });

  return (
    <motion.section
      ref={ref}
      initial={{ opacity: 0, y: 24 }}
      animate={isInView ? { opacity: 1, y: 0 } : {}}
      transition={{ duration: 0.6, ease: "easeOut" }}
      style={{
        background: "rgba(5, 8, 15, 0.95)",
        borderTop: "1px solid rgba(255,255,255,0.06)",
        borderBottom: "1px solid rgba(255,255,255,0.06)",
      }}
    >
      <div className="max-w-6xl mx-auto">
        <div className="flex flex-col sm:flex-row divide-y sm:divide-y-0">
          {stats.map((stat, i) => (
            <StatItem key={stat.label} stat={stat} active={isInView} index={i} />
          ))}
        </div>
      </div>
    </motion.section>
  );
}
