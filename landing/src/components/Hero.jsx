import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Github, ArrowRight, BookOpen, Calendar } from 'lucide-react';
import ParticleNetwork from '../utils/particleNetwork';
import { useTheme } from '../hooks/useTheme';

const USE_CASES = [
  'Run autonomous agents in isolated namespaces from a single AgentWorkload manifest.',
  'Cilium FQDN policies lock outbound traffic to approved destinations.',
  'Argo Workflows executes agent steps as observable DAGs with retries.',
  'MinIO stores artifacts, prompts, logs, and outputs per workload.',
  'Built for platform teams standardizing AI workloads on Kubernetes.',
];

const QUICKSTART_URL = 'https://github.com/Clawdlinux/agentic-operator-core/blob/main/docs/01-quickstart.md';
const DEMO_EMAIL_URL = 'mailto:007ssancheti@gmail.com?subject=Agentic%20Operator%20Demo%20Request';

const TERMINAL_LINES = [
  { prompt: '$ ', text: 'kubectl apply -f agentworkload.yaml', delay: 0 },
  { prompt: '', text: 'agentworkload.agentic.clawdlinux.io/research-run created', delay: 1200, teal: true },
  { prompt: '', text: '✓ namespace aw-research-run provisioned', delay: 2600, teal: true },
  { prompt: '', text: '✓ cilium policy restricted egress to github.com and api.openai.com', delay: 4100, teal: true },
  { prompt: '$ ', text: 'kubectl get agentworkload research-run -w', delay: 5600 },
  { prompt: '', text: '[reconcile] argo workflow started · minio bucket mounted', delay: 7100, teal: true },
  { prompt: '', text: '[ready] run complete · logs and artifacts retained for audit', delay: 8600, teal: true },
];

const headingVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.12 },
  },
};

const wordVariant = {
  hidden: { opacity: 0, y: 28, filter: 'blur(8px)' },
  visible: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { duration: 0.65, ease: [0.25, 0.46, 0.45, 0.94] },
  },
};

function TypingText({ text, speed = 38 }) {
  const [displayed, setDisplayed] = useState('');
  useEffect(() => {
    setDisplayed('');
    let i = 0;
    const id = setInterval(() => {
      if (i < text.length) {
        setDisplayed(text.slice(0, i + 1));
        i++;
      } else {
        clearInterval(id);
      }
    }, speed);
    return () => clearInterval(id);
  }, [text, speed]);
  return <>{displayed}</>;
}

const withAlpha = (hex, alpha) => `${hex}${alpha}`;

function TerminalWindow({ currentTheme, theme }) {
  const [visibleLines, setVisibleLines] = useState([]);
  const [typingIndex, setTypingIndex] = useState(0);
  const [typingText, setTypingText] = useState('');
  const [typingDone, setTypingDone] = useState(false);

  const terminalBg = theme === 'dark' ? '#0a0e1a' : '#f8fafc';
  const titlebarBg = currentTheme.bg.secondary;

  useEffect(() => {
    let timeouts = [];

    TERMINAL_LINES.forEach((line, idx) => {
      const t = setTimeout(() => {
        setTypingIndex(idx);
        setTypingText(line.text);
        setTypingDone(false);
        // After typing finishes, mark done
        const doneTimeout = setTimeout(() => {
          setVisibleLines((prev) => [...prev, { ...line, id: idx }]);
          setTypingDone(true);
        }, line.text.length * 38 + 150);
        timeouts.push(doneTimeout);
      }, line.delay);
      timeouts.push(t);
    });

    return () => timeouts.forEach(clearTimeout);
  }, []);

  return (
    <motion.div
      className="w-full max-w-2xl mx-auto rounded-xl overflow-hidden shadow-2xl shadow-black/60"
      initial={{ opacity: 0, y: 40 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.7, delay: 1.2, ease: [0.25, 0.46, 0.45, 0.94] }}
      style={{
        border: `1px solid ${currentTheme.border.light}`,
        boxShadow:
          theme === 'dark'
            ? '0 24px 80px rgba(0,0,0,0.55)'
            : '0 24px 60px rgba(15,23,42,0.16)',
      }}
    >
      {/* Traffic-light title bar */}
      <div
        className="flex items-center gap-2 px-4 py-3"
        style={{
          background: titlebarBg,
          borderBottom: `1px solid ${currentTheme.border.light}`,
        }}
      >
        <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
        <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
        <div className="w-3 h-3 rounded-full bg-[#28c840]" />
        <span
          className="ml-3 text-xs tracking-widest uppercase"
          style={{
            fontFamily: "'IBM Plex Mono', monospace",
            color: currentTheme.text.muted,
          }}
        >
          agentic-operator-core — kubernetes operator
        </span>
      </div>

      {/* Terminal body */}
      <div
        className="px-5 py-4 min-h-[220px] text-sm leading-relaxed"
        style={{
          fontFamily: "'IBM Plex Mono', monospace",
          background: terminalBg,
        }}
      >
        {visibleLines.map((line) => (
          <div key={line.id} className="mb-1">
            {line.multiline ? (
              line.text.split('\n').map((l, i) => (
                <div
                  key={i}
                  style={{ color: line.dim ? currentTheme.text.muted : currentTheme.text.secondary }}
                >
                  {i === 0 && <span style={{ color: currentTheme.accent.teal }}>{line.prompt}</span>}
                  {l}
                </div>
              ))
            ) : (
              <div
                style={{
                  color: line.teal
                    ? currentTheme.accent.teal
                    : line.dim
                    ? currentTheme.text.muted
                    : currentTheme.text.secondary,
                }}
              >
                {line.prompt && <span style={{ color: currentTheme.accent.teal }}>{line.prompt}</span>}
                {line.text}
              </div>
            )}
          </div>
        ))}

        {/* Currently typing line */}
        {!typingDone && typingText && (
          <div className="mb-1" style={{ color: currentTheme.text.secondary }}>
            {TERMINAL_LINES[typingIndex]?.prompt && (
              <span style={{ color: currentTheme.accent.teal }}>{TERMINAL_LINES[typingIndex].prompt}</span>
            )}
            <TypingText text={typingText} />
            <span
              className="inline-block w-2 h-4 ml-0.5 align-middle animate-pulse"
              style={{ background: currentTheme.accent.teal }}
            />
          </div>
        )}
      </div>
    </motion.div>
  );
}

export default function Hero() {
  const { currentTheme, theme } = useTheme();
  const canvasRef = useRef(null);
  const networkRef = useRef(null);
  const [useCaseIndex, setUseCaseIndex] = useState(0);

  // Particle canvas setup
  useEffect(() => {
    if (!canvasRef.current) return;
    const network = new ParticleNetwork(canvasRef.current, {
      count: 80,
      maxDistance: 150,
      speed: 0.3,
    });
    networkRef.current = network;
    network.start();

    return () => {
      network.stop();
    };
  }, []);

  // Rotating use cases
  useEffect(() => {
    const id = setInterval(() => {
      setUseCaseIndex((i) => (i + 1) % USE_CASES.length);
    }, 3000);
    return () => clearInterval(id);
  }, []);

  return (
    <section
      id="hero"
      className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden pt-16 lg:pt-18 transition-colors duration-300"
      style={{ background: currentTheme.bg.primary }}
    >
      {/* Particle canvas */}
      <canvas
        ref={canvasRef}
        className="absolute inset-0 w-full h-full pointer-events-none"
        style={{ zIndex: 0 }}
      />

      {/* Decorative gradient orbs */}
      <div
        className="absolute top-1/4 -left-32 w-96 h-96 rounded-full pointer-events-none"
        style={{
          background: `radial-gradient(circle, ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '1F' : '14')} 0%, rgba(0,212,170,0) 70%)`,
          animation: 'orbFloat 8s ease-in-out infinite',
          zIndex: 1,
        }}
      />
      <div
        className="absolute bottom-1/4 -right-32 w-[500px] h-[500px] rounded-full pointer-events-none"
        style={{
          background: `radial-gradient(circle, ${withAlpha(currentTheme.accent.indigo, theme === 'dark' ? '1A' : '12')} 0%, rgba(99,102,241,0) 70%)`,
          animation: 'orbFloat 10s ease-in-out infinite reverse',
          zIndex: 1,
        }}
      />
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[700px] h-[700px] rounded-full pointer-events-none"
        style={{
          background: `radial-gradient(circle, ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '0A' : '08')} 0%, rgba(0,212,170,0) 60%)`,
          zIndex: 1,
        }}
      />

      {/* Main content */}
      <div className="relative z-10 max-w-4xl mx-auto px-4 sm:px-6 text-center">

        {/* Badge */}
        <motion.div
          className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-medium mb-8"
          style={{ fontFamily: "'IBM Plex Mono', monospace" }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          whileHover={{ y: -1 }}
        >
          <div
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full"
            style={{
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              color: theme === 'dark' ? currentTheme.accent.teal : '#0b4f45',
            }}
          >
            <span className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: currentTheme.accent.teal }} />
            Kubernetes Operator · Open Source · Apache 2.0
          </div>
        </motion.div>

        {/* Animated heading */}
        <motion.div
          className="mb-6"
          variants={headingVariants}
          initial="hidden"
          animate="visible"
        >
          {/* Line 1 */}
          <h1
            className="text-5xl sm:text-6xl lg:text-7xl font-bold leading-tight tracking-tight"
            style={{ fontFamily: "'Syne', sans-serif" }}
          >
            <div className="flex flex-wrap justify-center gap-x-3 mb-2">
              {['Policy-Aware', 'Agent'].map((word) => (
                <motion.span
                  key={word}
                  variants={wordVariant}
                  className={word === 'Policy-Aware' ? 'text-gradient' : ''}
                  style={word === 'Policy-Aware' ? undefined : { color: currentTheme.text.primary }}
                >
                  {word}
                </motion.span>
              ))}
            </div>
            {/* Line 2 */}
            <div className="flex flex-wrap justify-center gap-x-3">
              {['Isolation', 'for', 'Kubernetes.'].map((word) => (
                <motion.span key={word} variants={wordVariant} style={{ color: currentTheme.text.primary }}>
                  {word}
                </motion.span>
              ))}
            </div>
          </h1>
        </motion.div>

        {/* Rotating subheadline */}
        <div className="h-12 flex items-center justify-center mb-8 overflow-hidden">
          <AnimatePresence mode="wait">
            <motion.p
              key={useCaseIndex}
              className="text-lg sm:text-xl max-w-2xl"
              style={{
                fontFamily: "'DM Sans', sans-serif",
                color: currentTheme.text.tertiary,
              }}
              initial={{ opacity: 0, y: 16, filter: 'blur(4px)' }}
              animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
              exit={{ opacity: 0, y: -16, filter: 'blur(4px)' }}
              transition={{ duration: 0.45, ease: 'easeInOut' }}
            >
              {USE_CASES[useCaseIndex]}
            </motion.p>
          </AnimatePresence>
        </div>

        {/* CTA buttons */}
        <motion.div
          className="flex flex-wrap items-center justify-center gap-4 mb-8"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.7 }}
        >
          <a
            href={QUICKSTART_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl transition-all duration-200 hover:brightness-110 active:scale-95"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              color: '#03231d',
              background: `linear-gradient(135deg, ${currentTheme.accent.teal} 0%, #00b894 100%)`,
              boxShadow: `0 10px 26px ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '3A' : '2A')}`,
            }}
          >
            <BookOpen className="w-4 h-4" />
            Start in 5 Minutes
            <ArrowRight className="w-4 h-4" />
          </a>
          <a
            href="https://github.com/Clawdlinux/agentic-operator-core"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl transition-all duration-200"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              color: currentTheme.text.secondary,
              border: `1px solid ${currentTheme.border.light}`,
              background: withAlpha(currentTheme.bg.secondary, theme === 'dark' ? '66' : 'AA'),
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = withAlpha(currentTheme.accent.teal, '66');
              e.currentTarget.style.color = currentTheme.accent.teal;
              e.currentTarget.style.background = withAlpha(currentTheme.accent.teal, theme === 'dark' ? '12' : '0D');
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = currentTheme.border.light;
              e.currentTarget.style.color = currentTheme.text.secondary;
              e.currentTarget.style.background = withAlpha(currentTheme.bg.secondary, theme === 'dark' ? '66' : 'AA');
            }}
          >
            <Github className="w-4 h-4" />
            View on GitHub
          </a>
          <a
            href={DEMO_EMAIL_URL}
            className="flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl transition-all duration-200"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              background: withAlpha(currentTheme.accent.indigo, theme === 'dark' ? '2E' : '20'),
              color: theme === 'dark' ? '#c7d2fe' : '#2b3672',
              border: `1px solid ${withAlpha(currentTheme.accent.indigo, '66')}`,
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = withAlpha(currentTheme.accent.indigo, theme === 'dark' ? '3D' : '2A');
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = withAlpha(currentTheme.accent.indigo, theme === 'dark' ? '2E' : '20');
            }}
          >
            <Calendar className="w-4 h-4" />
            Book Demo
          </a>
        </motion.div>

        {/* Technical capability ticker */}
        <motion.div
          className="flex items-center justify-center gap-2 mb-14"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.5, delay: 0.9 }}
        >
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
          </span>
          <span
            className="text-xs"
            style={{
              fontFamily: "'IBM Plex Mono', monospace",
              color: currentTheme.text.muted,
            }}
          >
            Apache 2.0 licensed · Argo DAG orchestration · Cilium egress guardrails
          </span>
        </motion.div>

        {/* Terminal window */}
        <TerminalWindow currentTheme={currentTheme} theme={theme} />
      </div>

      {/* Scroll indicator */}
      <motion.div
        className="absolute bottom-8 left-1/2 -translate-x-1/2 flex flex-col items-center gap-2"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.5, delay: 2 }}
        style={{ color: currentTheme.text.muted }}
      >
        <span className="text-xs" style={{ fontFamily: "'DM Sans', sans-serif" }}>
          scroll to explore
        </span>
        <motion.div
          className="w-px h-8"
          style={{
            background: `linear-gradient(to bottom, ${currentTheme.text.muted}, transparent)`,
          }}
          animate={{ scaleY: [1, 0.4, 1], opacity: [0.6, 1, 0.6] }}
          transition={{ duration: 1.8, repeat: Infinity, ease: 'easeInOut' }}
        />
      </motion.div>

      {/* Keyframe animations injected via style tag */}
      <style>{`
        @keyframes orbFloat {
          0%, 100% { transform: translateY(0px) scale(1); }
          50% { transform: translateY(-30px) scale(1.05); }
        }
        .text-gradient {
          background: linear-gradient(135deg, #00d4aa 0%, #00b894 50%, #7c3aed 100%);
          -webkit-background-clip: text;
          -webkit-text-fill-color: transparent;
          background-clip: text;
        }
      `}</style>
    </section>
  );
}
