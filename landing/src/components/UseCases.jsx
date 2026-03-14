import { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence, useInView } from "framer-motion";
import { ArrowRight, TrendingDown, DollarSign, Layers, Briefcase } from "lucide-react";

const YAML_PRICING = `# vmi-config: pricing-tracker.yaml
targets:
  - name: stripe-pricing
    url: https://stripe.com/pricing
    schedule: "0 8 * * *"    # Daily at 8am
    regions: [us-east, eu-west]
    
  - name: competitor-a-plans
    url: https://competitor-a.com/pricing
    schedule: "0 8 * * *"

analysis:
  engine: claude-sonnet-4
  detect: [price-changes, new-tiers, removed-features]
  alert_threshold: any_change
  
output:
  format: pdf
  deliver_to: [slack:#competitive-intel, email:team@company.com]`;

const YAML_FEATURE = `# vmi-config: feature-tracker.yaml
targets:
  - name: competitor-product-page  
    url: https://competitor.com/product
    capture: [full-page, above-fold]
    schedule: "*/6 * * * *"  # Every 6 hours
    
  - name: competitor-changelog
    url: https://competitor.com/changelog
    schedule: "0 */4 * * *"

analysis:
  engine: claude-sonnet-4
  detect: [new-features, ui-changes, messaging-shifts]
  compare_with: previous_7_days
  
alerts:
  channels: [slack, email]
  urgency: high_for_pricing | medium_for_features`;

const YAML_PORTFOLIO = `# vmi-config: portfolio-monitor.yaml
portfolio:
  - company: Series-B Target
    pages:
      - url: https://target.com
      - url: https://target.com/pricing  
      - url: https://target.com/about
    schedule: "0 9 * * MON"  # Weekly Monday

  - company: Portfolio Company #3
    pages:
      - url: https://portfolio3.com
      - url: https://portfolio3.com/product

analysis:
  engine: claude-sonnet-4
  track: [growth-signals, team-changes, product-velocity]
  benchmark_against: industry_median
  
reports:
  format: pdf
  frequency: weekly
  deliver_to: [email:partners@fund.com]`;

const yamlMap = { YAML_PRICING, YAML_FEATURE, YAML_PORTFOLIO };

const tabs = [
  {
    id: "pricing",
    label: "Pricing Intelligence",
    icon: DollarSign,
    problem:
      "Competitors change pricing without warning. Your team finds out weeks later from a customer complaint \u2014 after you\u2019ve already lost deals.",
    solution:
      "VMI monitors competitor pricing pages daily, detects any change within hours, and delivers AI-analyzed reports explaining what changed and what it means for your positioning.",
    before: { value: "2-3 weeks", label: "Time to detect pricing changes" },
    after: { value: "<4 hours", label: "Automated detection" },
    savings: { value: "Revenue protected", color: "#f59e0b" },
    yamlKey: "YAML_PRICING",
    configFile: "pricing-tracker.yaml",
  },
  {
    id: "features",
    label: "Feature Launch Tracking",
    icon: Layers,
    problem:
      "Competitors ship new features and you only find out from your sales team asking \u2018did you see what they launched?\u2019 You\u2019re always playing catch-up.",
    solution:
      "VMI captures competitor product pages every 6 hours, uses AI vision to detect new features, UI changes, and messaging shifts \u2014 then alerts your team instantly.",
    before: { value: "Days to weeks", label: "Manual discovery of competitor launches" },
    after: { value: "Same day", label: "Automated alerts with AI analysis" },
    savings: { value: "Competitive edge maintained", color: "#00d4aa" },
    yamlKey: "YAML_FEATURE",
    configFile: "feature-tracker.yaml",
  },
  {
    id: "portfolio",
    label: "Portfolio Monitoring",
    icon: Briefcase,
    problem:
      "VC analysts spend 6+ hours per company on manual competitive landscape research. With 20+ portfolio companies, due diligence is perpetually outdated.",
    solution:
      "VMI continuously monitors portfolio companies and their competitors, delivering weekly structured reports tracking growth signals, product velocity, and market positioning.",
    before: { value: "6+ hours/company", label: "Manual research per due diligence" },
    after: { value: "Automated weekly", label: "Always-current intelligence" },
    savings: { value: "100+ analyst hours saved/month", color: "#6366f1" },
    yamlKey: "YAML_PORTFOLIO",
    configFile: "portfolio-monitor.yaml",
  },
];

function MetricCard({ label, value, sublabel, highlight, highlightColor }) {
  return (
    <div
      className="flex-1 rounded-xl p-5"
      style={{
        background: highlight
          ? `rgba(${highlightColor}, 0.08)`
          : "rgba(255,255,255,0.03)",
        border: highlight
          ? `1px solid rgba(${highlightColor}, 0.25)`
          : "1px solid rgba(255,255,255,0.06)",
      }}
    >
      <div
        className="text-xs font-semibold uppercase tracking-widest mb-3"
        style={{
          fontFamily: "'DM Sans', sans-serif",
          color: highlight ? `rgb(${highlightColor})` : "#94a3b8",
        }}
      >
        {label}
      </div>
      <div
        className="text-2xl font-bold mb-1"
        style={{
          fontFamily: "'Syne', sans-serif",
          color: highlight ? `rgb(${highlightColor})` : "#e2e8f0",
        }}
      >
        {value}
      </div>
      <div
        className="text-sm"
        style={{
          fontFamily: "'DM Sans', sans-serif",
          color: "#94a3b8",
        }}
      >
        {sublabel}
      </div>
    </div>
  );
}

function CodeBlock({ code, title }) {
  return (
    <div
      className="rounded-xl overflow-hidden"
      style={{
        background: "rgba(5, 8, 15, 0.9)",
        border: "1px solid rgba(255,255,255,0.08)",
      }}
    >
      <div
        className="flex items-center gap-2 px-4 py-3"
        style={{
          background: "rgba(255,255,255,0.03)",
          borderBottom: "1px solid rgba(255,255,255,0.06)",
        }}
      >
        <div className="w-3 h-3 rounded-full" style={{ background: "#ff5f57" }} />
        <div className="w-3 h-3 rounded-full" style={{ background: "#ffbd2e" }} />
        <div className="w-3 h-3 rounded-full" style={{ background: "#28c840" }} />
        <span
          className="ml-2 text-xs"
          style={{ fontFamily: "'IBM Plex Mono', monospace", color: "#94a3b8" }}
        >
          {title}
        </span>
      </div>
      <pre
        className="p-5 text-xs leading-relaxed overflow-x-auto"
        style={{
          fontFamily: "'IBM Plex Mono', monospace",
          color: "#e2e8f0",
          margin: 0,
        }}
      >
        <code
          dangerouslySetInnerHTML={{
            __html: (() => {
              const escaped = code
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');
              return escaped
                .replace(/^(\s*#.*)$/gm, '<span style="color:#64748b">$1</span>')
                .replace(/"([^"]*)"/g, '<span style="color:#f59e0b">"$1"</span>')
                .replace(/\b(true|false)\b/g, '<span style="color:#f59e0b">$1</span>')
                .replace(/(targets:|analysis:|output:|alerts:|portfolio:|reports:|pages:|schedule:|name:|url:|engine:|detect:|alert_threshold:|capture:|compare_with:|channels:|urgency:|company:|track:|benchmark_against:|frequency:|deliver_to:|format:|regions:)/g,
                  '<span style="color:#6366f1">$1</span>')
                .replace(/(claude-sonnet-4)/g,
                  '<span style="color:#00d4aa">$1</span>');
            })(),
          }}
        />
      </pre>
    </div>
  );
}

export default function UseCases() {
  const [activeTab, setActiveTab] = useState(0);
  const ref = useRef(null);

  const tab = tabs[activeTab];

  return (
    <section
      id="use-cases"
      className="py-24 px-4"
      style={{
        background: "linear-gradient(180deg, #05080f 0%, #070b14 100%)",
      }}
    >
      <div className="max-w-5xl mx-auto">
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="text-center mb-14"
        >
          <div
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-semibold uppercase tracking-widest mb-6"
            style={{
              background: "rgba(99, 102, 241, 0.08)",
              border: "1px solid rgba(99, 102, 241, 0.2)",
              color: "#6366f1",
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            Use Cases
          </div>
          <h2
            className="text-3xl sm:text-4xl lg:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: "#e2e8f0",
            }}
          >
            Three Ways Teams Use{" "}
            <span
              style={{
                background: "linear-gradient(135deg, #00d4aa, #6366f1)",
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
              }}
            >
              VMI
            </span>
          </h2>
        </motion.div>

        {/* Tab Navigation */}
        <div
          className="flex flex-col sm:flex-row gap-1 p-1 rounded-xl mb-10"
          style={{
            background: "rgba(13, 21, 37, 0.7)",
            border: "1px solid rgba(255,255,255,0.06)",
          }}
        >
          {tabs.map((t, i) => (
            <button
              key={t.id}
              onClick={() => setActiveTab(i)}
              className="flex-1 px-4 py-3 rounded-lg text-sm font-medium transition-all duration-300 text-left sm:text-center relative"
              style={{
                fontFamily: "'DM Sans', sans-serif",
                color: activeTab === i ? "#e2e8f0" : "#94a3b8",
                background: activeTab === i ? "rgba(0, 212, 170, 0.1)" : "transparent",
                border: activeTab === i ? "1px solid rgba(0, 212, 170, 0.25)" : "1px solid transparent",
              }}
            >
              {activeTab === i && (
                <motion.span
                  layoutId="tab-indicator"
                  className="absolute inset-0 rounded-lg"
                  style={{
                    background: "rgba(0, 212, 170, 0.08)",
                  }}
                  transition={{ type: "spring", bounce: 0.2, duration: 0.5 }}
                />
              )}
              <span className="relative z-10">{t.label}</span>
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -16 }}
            transition={{ duration: 0.35, ease: "easeOut" }}
          >
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
              {/* Left: Problem / Solution / Metrics */}
              <div className="flex flex-col gap-6">
                <div
                  className="rounded-xl p-6"
                  style={{
                    background: "rgba(13, 21, 37, 0.7)",
                    border: "1px solid rgba(255,255,255,0.06)",
                  }}
                >
                  <div
                    className="text-xs font-semibold uppercase tracking-widest mb-3"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#ef4444",
                    }}
                  >
                    The Problem
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#94a3b8",
                    }}
                  >
                    {tab.problem}
                  </p>
                </div>

                <div
                  className="rounded-xl p-6"
                  style={{
                    background: "rgba(13, 21, 37, 0.7)",
                    border: "1px solid rgba(0, 212, 170, 0.15)",
                  }}
                >
                  <div
                    className="text-xs font-semibold uppercase tracking-widest mb-3"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#00d4aa",
                    }}
                  >
                    The Solution
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: "#e2e8f0",
                    }}
                  >
                    {tab.solution}
                  </p>
                </div>

                {/* Metrics */}
                <div className="flex gap-4">
                  <MetricCard
                    label="Before"
                    value={tab.before.value}
                    sublabel={tab.before.label}
                    highlight={false}
                  />
                  <div className="flex items-center">
                    <ArrowRight size={20} color="#94a3b8" />
                  </div>
                  <MetricCard
                    label="After"
                    value={tab.after.value}
                    sublabel={tab.after.label}
                    highlight={true}
                    highlightColor="0, 212, 170"
                  />
                </div>

                <div
                  className="rounded-xl px-5 py-4 flex items-center gap-3"
                  style={{
                    background: `rgba(${tab.savings.color === "#f59e0b" ? "245, 158, 11" : tab.savings.color === "#00d4aa" ? "0, 212, 170" : "99, 102, 241"}, 0.08)`,
                    border: `1px solid rgba(${tab.savings.color === "#f59e0b" ? "245, 158, 11" : tab.savings.color === "#00d4aa" ? "0, 212, 170" : "99, 102, 241"}, 0.2)`,
                  }}
                >
                  <TrendingDown size={18} color={tab.savings.color} />
                  <span
                    className="text-sm font-semibold"
                    style={{
                      fontFamily: "'DM Sans', sans-serif",
                      color: tab.savings.color,
                    }}
                  >
                    {tab.savings.value}
                  </span>
                </div>
              </div>

              {/* Right: Code Snippet */}
              <div>
                <CodeBlock code={yamlMap[tab.yamlKey]} title={tab.configFile} />
              </div>
            </div>
          </motion.div>
        </AnimatePresence>
      </div>
    </section>
  );
}
