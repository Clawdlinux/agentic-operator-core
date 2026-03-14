import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Eye, ArrowRight } from 'lucide-react';
import ParticleNetwork from '../utils/particleNetwork';

const USE_CASES = [
  'Daily visual snapshots of every competitor page that matters.',
  'AI detects pricing changes, new features, and messaging shifts overnight.',
  'Structured PDF reports delivered to your inbox before standup.',
  'Track 50+ competitor pages — zero manual effort.',
  'From screenshot to strategic insight in under 5 minutes.',
];

const TERMINAL_LINES = [
  { prompt: '$ ', text: 'vmi scan --targets competitors.yaml', delay: 0 },
  { prompt: '', text: 'Scanning 47 competitor pages...', delay: 1200, dim: true },
  { prompt: '', text: '✓ 47/47 screenshots captured (12.3s)', delay: 2400, teal: true },
  { prompt: '', text: '✓ 8 visual changes detected across 5 competitors', delay: 3600, teal: true },
  { prompt: '$ ', text: 'vmi analyze --diff --ai-summary', delay: 5200 },
  { prompt: '', text: '[AI] Stripe raised Enterprise pricing 18% · New "Scale" tier added', delay: 6400, teal: true },
  { prompt: '', text: '[AI] Report generated → /reports/competitive-intel-jun-2025.pdf', delay: 7600, teal: true },
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

function TerminalWindow() {
  const [visibleLines, setVisibleLines] = useState([]);
  const [typingIndex, setTypingIndex] = useState(0);
  const [typingText, setTypingText] = useState('');
  const [typingDone, setTypingDone] = useState(false);

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
      style={{ border: '1px solid rgba(255,255,255,0.08)' }}
    >
      {/* Traffic-light title bar */}
      <div className="flex items-center gap-2 px-4 py-3 bg-[#0d1117] border-b border-white/5">
        <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
        <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
        <div className="w-3 h-3 rounded-full bg-[#28c840]" />
        <span
          className="ml-3 text-xs text-slate-500 tracking-widest uppercase"
          style={{ fontFamily: "'IBM Plex Mono', monospace" }}
        >
          vmi — competitive intelligence
        </span>
      </div>

      {/* Terminal body */}
      <div
        className="bg-[#0a0e1a] px-5 py-4 min-h-[220px] text-sm leading-relaxed"
        style={{ fontFamily: "'IBM Plex Mono', monospace" }}
      >
        {visibleLines.map((line) => (
          <div key={line.id} className="mb-1">
            {line.multiline ? (
              line.text.split('\n').map((l, i) => (
                <div key={i} className={line.dim ? 'text-slate-500' : 'text-slate-300'}>
                  {i === 0 && <span className="text-[#00d4aa]">{line.prompt}</span>}
                  {l}
                </div>
              ))
            ) : (
              <div className={line.teal ? 'text-[#00d4aa]' : line.dim ? 'text-slate-500' : 'text-slate-300'}>
                {line.prompt && <span className="text-[#00d4aa]">{line.prompt}</span>}
                {line.text}
              </div>
            )}
          </div>
        ))}

        {/* Currently typing line */}
        {!typingDone && typingText && (
          <div className="mb-1 text-slate-300">
            {TERMINAL_LINES[typingIndex]?.prompt && (
              <span className="text-[#00d4aa]">{TERMINAL_LINES[typingIndex].prompt}</span>
            )}
            <TypingText text={typingText} />
            <span className="inline-block w-2 h-4 bg-[#00d4aa] ml-0.5 align-middle animate-pulse" />
          </div>
        )}
      </div>
    </motion.div>
  );
}

export default function Hero() {
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
      className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden pt-16 lg:pt-18"
      style={{ background: '#05080f' }}
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
          background: 'radial-gradient(circle, rgba(0,212,170,0.12) 0%, rgba(0,212,170,0) 70%)',
          animation: 'orbFloat 8s ease-in-out infinite',
          zIndex: 1,
        }}
      />
      <div
        className="absolute bottom-1/4 -right-32 w-[500px] h-[500px] rounded-full pointer-events-none"
        style={{
          background: 'radial-gradient(circle, rgba(139,92,246,0.10) 0%, rgba(139,92,246,0) 70%)',
          animation: 'orbFloat 10s ease-in-out infinite reverse',
          zIndex: 1,
        }}
      />
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[700px] h-[700px] rounded-full pointer-events-none"
        style={{
          background: 'radial-gradient(circle, rgba(0,212,170,0.04) 0%, rgba(0,212,170,0) 60%)',
          zIndex: 1,
        }}
      />

      {/* Main content */}
      <div className="relative z-10 max-w-4xl mx-auto px-4 sm:px-6 text-center">

        {/* Badge */}
        <motion.div
          className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full border border-[#00d4aa]/25 bg-[#00d4aa]/5 text-[#00d4aa] text-xs font-medium mb-8"
          style={{ fontFamily: "'IBM Plex Mono', monospace" }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
        >
          <span className="w-1.5 h-1.5 rounded-full bg-[#00d4aa] animate-pulse" />
          Competitive Intelligence · AI-Powered · Automated
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
              {['See', 'What', 'Your'].map((word) => (
                <motion.span
                  key={word}
                  variants={wordVariant}
                  className={word === 'What' || word === 'Your' ? 'text-gradient' : 'text-white'}
                >
                  {word}
                </motion.span>
              ))}
            </div>
            {/* Line 2 */}
            <div className="flex flex-wrap justify-center gap-x-3">
              {['Competitors', 'Change.'].map((word) => (
                <motion.span key={word} variants={wordVariant} className="text-white">
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
              className="text-lg sm:text-xl text-slate-400 max-w-2xl"
              style={{ fontFamily: "'DM Sans', sans-serif" }}
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
            href="#architecture"
            onClick={(e) => {
              e.preventDefault();
              document.querySelector('#architecture')?.scrollIntoView({ behavior: 'smooth' });
            }}
            className="flex items-center gap-2 px-6 py-3 text-sm font-semibold text-[#00d4aa] rounded-xl border border-[#00d4aa]/40 hover:bg-[#00d4aa]/8 hover:border-[#00d4aa]/70 transition-all duration-200 hover:shadow-lg hover:shadow-[#00d4aa]/10 active:scale-95"
            style={{ fontFamily: "'DM Sans', sans-serif" }}
          >
            <Eye className="w-4 h-4" />
            Watch Demo
          </a>

          <a
            href="#waitlist"
            onClick={(e) => {
              e.preventDefault();
              document.querySelector('#waitlist')?.scrollIntoView({ behavior: 'smooth' });
            }}
            className="flex items-center gap-2 px-6 py-3 text-sm font-semibold text-[#05080f] rounded-xl transition-all duration-200 hover:brightness-110 hover:shadow-xl hover:shadow-[#00d4aa]/25 active:scale-95"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
            }}
          >
            Get Early Access
            <ArrowRight className="w-4 h-4" />
          </a>
        </motion.div>

        {/* Production stats ticker */}
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
            className="text-xs text-slate-500"
            style={{ fontFamily: "'IBM Plex Mono', monospace" }}
          >
            47 competitor pages tracked · 8 changes detected today · Reports in &lt;5 min
          </span>
        </motion.div>

        {/* Terminal window */}
        <TerminalWindow />
      </div>

      {/* Scroll indicator */}
      <motion.div
        className="absolute bottom-8 left-1/2 -translate-x-1/2 flex flex-col items-center gap-2 text-slate-600"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.5, delay: 2 }}
      >
        <span className="text-xs" style={{ fontFamily: "'DM Sans', sans-serif" }}>
          scroll to explore
        </span>
        <motion.div
          className="w-px h-8 bg-gradient-to-b from-slate-600 to-transparent"
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
