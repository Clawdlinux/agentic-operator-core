import { useState, useEffect } from 'react';
import { motion, useScroll, useTransform } from 'framer-motion';
import { Menu, X, Star, ExternalLink, Hexagon } from 'lucide-react';

const NAV_LINKS = [
  { label: 'Features', href: '#features' },
  { label: 'Use Cases', href: '#use-cases' },
  { label: 'GitHub', href: 'https://github.com/shreyanshjain7174/agentic-k8s-operator', external: true },
  { label: 'Docs', href: 'https://github.com/shreyanshjain7174/agentic-k8s-operator#readme', external: true },
];

const GITHUB_URL = 'https://github.com/shreyanshjain7174/agentic-k8s-operator';

export default function Navigation() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const { scrollY } = useScroll();

  const bgOpacity = useTransform(scrollY, [0, 50], [0, 1]);
  const blurAmount = useTransform(scrollY, [0, 50], [0, 12]);

  useEffect(() => {
    const unsubscribe = scrollY.on('change', (y) => {
      setScrolled(y > 50);
    });
    return () => unsubscribe();
  }, [scrollY]);

  const handleSmoothScroll = (e, href) => {
    if (href.startsWith('#')) {
      e.preventDefault();
      const el = document.querySelector(href);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth' });
        setMenuOpen(false);
      }
    }
  };

  return (
    <motion.nav
      className="fixed top-0 left-0 right-0 z-50 transition-all duration-300"
      style={{
        backdropFilter: scrolled ? 'blur(12px)' : 'blur(0px)',
        WebkitBackdropFilter: scrolled ? 'blur(12px)' : 'blur(0px)',
      }}
    >
      {/* Glass background layer */}
      <motion.div
        className="absolute inset-0 bg-[#05080f] border-b border-white/5"
        style={{ opacity: bgOpacity }}
      />

      <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16 lg:h-18">

          {/* Logo */}
          <motion.a
            href="/"
            className="flex items-center gap-2.5 group"
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.5 }}
          >
            <div className="relative">
              <Hexagon
                className="w-8 h-8 text-[#00d4aa] transition-transform duration-300 group-hover:rotate-12"
                strokeWidth={1.5}
                fill="rgba(0,212,170,0.1)"
              />
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="w-2 h-2 rounded-full bg-[#00d4aa] opacity-80" />
              </div>
            </div>
            <span
              className="text-white font-semibold text-lg tracking-tight"
              style={{ fontFamily: "'Syne', sans-serif" }}
            >
              Agentic<span className="text-[#00d4aa]">Operator</span>
            </span>
          </motion.a>

          {/* Desktop Nav Links */}
          <motion.div
            className="hidden md:flex items-center gap-1"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.1 }}
          >
            {NAV_LINKS.map((link, i) => (
              <a
                key={link.label}
                href={link.href}
                target={link.external ? '_blank' : undefined}
                rel={link.external ? 'noopener noreferrer' : undefined}
                onClick={!link.external ? (e) => handleSmoothScroll(e, link.href) : undefined}
                className="flex items-center gap-1 px-4 py-2 text-sm text-slate-400 hover:text-white rounded-lg hover:bg-white/5 transition-all duration-200 font-medium"
                style={{ fontFamily: "'DM Sans', sans-serif" }}
              >
                {link.label}
                {link.external && <ExternalLink className="w-3 h-3 opacity-60" />}
              </a>
            ))}
          </motion.div>

          {/* Desktop CTA buttons */}
          <motion.div
            className="hidden md:flex items-center gap-3"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.5, delay: 0.15 }}
          >
            {/* GitHub Star button */}
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 px-4 py-2 text-sm text-slate-300 border border-white/10 rounded-lg hover:border-[#00d4aa]/40 hover:text-[#00d4aa] transition-all duration-200 bg-white/5 hover:bg-[#00d4aa]/5"
              style={{ fontFamily: "'DM Sans', sans-serif" }}
            >
              <Star className="w-3.5 h-3.5" />
              <span>Star</span>
            </a>

            {/* Get Early Access CTA */}
            <a
              href="#waitlist"
              onClick={(e) => handleSmoothScroll(e, '#waitlist')}
              className="flex items-center gap-2 px-5 py-2 text-sm font-semibold text-[#05080f] rounded-lg transition-all duration-200 hover:brightness-110 hover:shadow-lg hover:shadow-[#00d4aa]/20 active:scale-95"
              style={{
                fontFamily: "'DM Sans', sans-serif",
                background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
              }}
            >
              Get Early Access
            </a>
          </motion.div>

          {/* Mobile hamburger */}
          <motion.button
            className="md:hidden p-2 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition-all duration-200"
            onClick={() => setMenuOpen((v) => !v)}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.5 }}
            aria-label={menuOpen ? 'Close menu' : 'Open menu'}
          >
            {menuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
          </motion.button>
        </div>
      </div>

      {/* Mobile menu */}
      <motion.div
        className="md:hidden overflow-hidden"
        initial={false}
        animate={{ height: menuOpen ? 'auto' : 0, opacity: menuOpen ? 1 : 0 }}
        transition={{ duration: 0.25, ease: 'easeInOut' }}
      >
        <div
          className="relative border-t border-white/5 bg-[#05080f]/95 backdrop-blur-xl px-4 py-4 flex flex-col gap-1"
        >
          {NAV_LINKS.map((link) => (
            <a
              key={link.label}
              href={link.href}
              target={link.external ? '_blank' : undefined}
              rel={link.external ? 'noopener noreferrer' : undefined}
              onClick={!link.external ? (e) => handleSmoothScroll(e, link.href) : undefined}
              className="flex items-center justify-between px-4 py-3 text-sm text-slate-400 hover:text-white rounded-lg hover:bg-white/5 transition-all duration-200"
              style={{ fontFamily: "'DM Sans', sans-serif" }}
            >
              <span>{link.label}</span>
              {link.external && <ExternalLink className="w-3.5 h-3.5 opacity-60" />}
            </a>
          ))}

          <div className="mt-3 pt-3 border-t border-white/5 flex flex-col gap-2">
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2 px-4 py-3 text-sm text-slate-300 border border-white/10 rounded-lg hover:border-[#00d4aa]/40 hover:text-[#00d4aa] transition-all duration-200"
              style={{ fontFamily: "'DM Sans', sans-serif" }}
            >
              <Star className="w-4 h-4" />
              <span>Star on GitHub</span>
            </a>
            <a
              href="#waitlist"
              onClick={(e) => handleSmoothScroll(e, '#waitlist')}
              className="flex items-center justify-center px-4 py-3 text-sm font-semibold text-[#05080f] rounded-lg"
              style={{
                fontFamily: "'DM Sans', sans-serif",
                background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
              }}
            >
              Get Early Access
            </a>
          </div>
        </div>
      </motion.div>
    </motion.nav>
  );
}
