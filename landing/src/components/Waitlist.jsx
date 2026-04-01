import { useState } from 'react';
import { motion } from 'framer-motion';
import { BookOpen, Github, Mail, Send } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

const CONTACT_FORM_URL = import.meta.env.VITE_CONTACT_FORM_URL?.trim();
const CONTACT_FALLBACK_EMAIL = import.meta.env.VITE_CONTACT_FALLBACK_EMAIL?.trim() || '007ssancheti@gmail.com';

export default function Waitlist() {
  const { currentTheme, theme } = useTheme();
  const withAlpha = (hex, alpha) => `${hex}${alpha}`;

  const [form, setForm] = useState({
    name: '',
    email: '',
    company: '',
    message: '',
  });
  const [status, setStatus] = useState({ type: 'idle', message: '' });

  const inputStyle = {
    width: '100%',
    background:
      theme === 'dark'
        ? withAlpha(currentTheme.bg.primary, 'C7')
        : withAlpha(currentTheme.bg.primary, 'F0'),
    border: `1px solid ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '33' : '26')}`,
    borderRadius: 12,
    color: currentTheme.text.primary,
    padding: '12px 14px',
    fontFamily: "'DM Sans', sans-serif",
    outline: 'none',
  };

  const openMailFallback = (payload) => {
    const subject = `Agentic Operator Pilot Request: ${payload.company || payload.name}`;
    const body = [
      `Name: ${payload.name}`,
      `Email: ${payload.email}`,
      `Company: ${payload.company || 'N/A'}`,
      '',
      payload.message,
    ].join('\n');
    const href = `mailto:${CONTACT_FALLBACK_EMAIL}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
    window.location.href = href;
  };

  const onChange = (event) => {
    const { name, value } = event.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  const onSubmit = async (event) => {
    event.preventDefault();

    if (!form.name.trim() || !form.email.trim() || !form.message.trim()) {
      setStatus({ type: 'error', message: 'Name, email, and request details are required.' });
      return;
    }

    const payload = {
      name: form.name.trim(),
      email: form.email.trim(),
      company: form.company.trim(),
      message: form.message.trim(),
      source: 'landing-contact',
      submittedAt: new Date().toISOString(),
      page: typeof window !== 'undefined' ? window.location.href : 'unknown',
    };

    setStatus({ type: 'sending', message: 'Sending request...' });

    if (CONTACT_FORM_URL) {
      try {
        const formBody = new URLSearchParams(payload).toString();
        const response = await fetch(CONTACT_FORM_URL, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
          },
          body: formBody,
        });

        if (!response.ok && response.type !== 'opaque') {
          throw new Error(`Request failed with status ${response.status}`);
        }

        setForm({ name: '', email: '', company: '', message: '' });
        setStatus({ type: 'success', message: 'Request submitted. We will follow up via email.' });
        return;
      } catch (error) {
        console.error('Contact webhook failed, using email fallback:', error);
      }
    }

    openMailFallback(payload);
    setStatus({
      type: 'success',
      message: 'Opened your email client with a prefilled request. Send it to complete submission.',
    });
  };

  const cardStyle = {
    background:
      theme === 'dark'
        ? withAlpha(currentTheme.bg.secondary, 'BF')
        : withAlpha(currentTheme.bg.secondary, 'E6'),
    border: `1px solid ${withAlpha(currentTheme.accent.teal, '26')}`,
    backdropFilter: 'blur(16px)',
    boxShadow: theme === 'dark' ? '0 8px 60px rgba(0,0,0,0.5)' : '0 8px 38px rgba(15,23,42,0.14)',
  };

  return (
    <section
      id="waitlist"
      style={{ background: currentTheme.bg.primary }}
      className="py-24 px-6 overflow-hidden relative"
    >
      <div
        className="absolute pointer-events-none"
        style={{
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: 600,
          height: 600,
          borderRadius: '50%',
          background: `radial-gradient(circle, ${withAlpha(currentTheme.accent.teal, theme === 'dark' ? '12' : '0D')} 0%, transparent 70%)`,
          filter: 'blur(40px)',
        }}
      />

      <motion.div
        className="max-w-4xl mx-auto relative z-10"
        initial={{ opacity: 0, y: 24 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, amount: 0.2 }}
        transition={{ duration: 0.6, ease: 'easeOut' }}
      >
        <div className="text-center mb-10">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: theme === 'dark' ? currentTheme.accent.teal : '#0b4f45',
              background: withAlpha(currentTheme.accent.teal, theme === 'dark' ? '14' : '10'),
              border: `1px solid ${withAlpha(currentTheme.accent.teal, '40')}`,
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Contact
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold"
            style={{
              fontFamily: "'Syne', sans-serif",
              color: currentTheme.text.primary,
            }}
          >
            Request a Pilot
          </h2>
          <p
            className="text-center text-lg mt-4 max-w-2xl mx-auto"
            style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
          >
            Submit your use case and we will reply with architecture guidance, rollout sequencing, and the right onboarding path.
          </p>
        </div>

        <div className="grid lg:grid-cols-5 gap-5">
          <form
            onSubmit={onSubmit}
            className="lg:col-span-3 rounded-2xl p-6"
            style={cardStyle}
          >
            <div className="grid md:grid-cols-2 gap-4">
              <label className="block">
                <span
                  className="text-xs font-semibold tracking-wider uppercase"
                  style={{ color: currentTheme.text.tertiary, fontFamily: "'IBM Plex Mono', monospace" }}
                >
                  Name
                </span>
                <input
                  type="text"
                  name="name"
                  value={form.name}
                  onChange={onChange}
                  style={inputStyle}
                  className="mt-2"
                  autoComplete="name"
                  required
                />
              </label>

              <label className="block">
                <span
                  className="text-xs font-semibold tracking-wider uppercase"
                  style={{ color: currentTheme.text.tertiary, fontFamily: "'IBM Plex Mono', monospace" }}
                >
                  Work Email
                </span>
                <input
                  type="email"
                  name="email"
                  value={form.email}
                  onChange={onChange}
                  style={inputStyle}
                  className="mt-2"
                  autoComplete="email"
                  required
                />
              </label>
            </div>

            <label className="block mt-4">
              <span
                className="text-xs font-semibold tracking-wider uppercase"
                style={{ color: currentTheme.text.tertiary, fontFamily: "'IBM Plex Mono', monospace" }}
              >
                Company
              </span>
              <input
                type="text"
                name="company"
                value={form.company}
                onChange={onChange}
                style={inputStyle}
                className="mt-2"
                autoComplete="organization"
              />
            </label>

            <label className="block mt-4">
              <span
                className="text-xs font-semibold tracking-wider uppercase"
                style={{ color: currentTheme.text.tertiary, fontFamily: "'IBM Plex Mono', monospace" }}
              >
                What do you want to deploy?
              </span>
              <textarea
                name="message"
                value={form.message}
                onChange={onChange}
                style={{ ...inputStyle, minHeight: 128, resize: 'vertical' }}
                className="mt-2"
                placeholder="Describe workload scale, security constraints, and desired timeline."
                required
              />
            </label>

            <div className="flex flex-wrap items-center gap-3 mt-5">
              <button
                type="submit"
                disabled={status.type === 'sending'}
                className="inline-flex items-center gap-2 px-5 py-3 rounded-xl text-sm font-semibold transition-all duration-200 disabled:opacity-60 disabled:cursor-not-allowed"
                style={{
                  background: `linear-gradient(90deg, ${withAlpha(currentTheme.accent.teal, 'E6')}, ${withAlpha(currentTheme.accent.indigo, 'D9')})`,
                  color: '#031312',
                  fontFamily: "'IBM Plex Mono', monospace",
                }}
              >
                <Send size={15} />
                {status.type === 'sending' ? 'Sending...' : 'Send Request'}
              </button>

              <a
                href={`mailto:${CONTACT_FALLBACK_EMAIL}`}
                className="inline-flex items-center gap-2 px-5 py-3 rounded-xl text-sm font-semibold transition-all duration-200"
                style={{
                  border: `1px solid ${withAlpha(currentTheme.accent.teal, '4D')}`,
                  color: currentTheme.text.primary,
                  fontFamily: "'IBM Plex Mono', monospace",
                  textDecoration: 'none',
                }}
              >
                <Mail size={15} />
                Email Directly
              </a>
            </div>

            {status.type !== 'idle' && (
              <p
                className="mt-4 text-sm"
                style={{
                  color: status.type === 'error' ? '#fb7185' : currentTheme.accent.teal,
                  fontFamily: "'DM Sans', sans-serif",
                }}
                aria-live="polite"
              >
                {status.message}
              </p>
            )}
          </form>

          <div className="lg:col-span-2 grid gap-5">
            <a
              href="https://github.com/Clawdlinux/agentic-operator-core"
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-2xl p-6 transition-all duration-200 hover:-translate-y-1"
              style={cardStyle}
            >
              <Github className="w-6 h-6 mb-4" style={{ color: currentTheme.accent.teal }} />
              <h3
                className="text-lg font-semibold mb-2"
                style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
              >
                Explore the Repo
              </h3>
              <p style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
                Review CRDs, controllers, agents, and deployment assets directly in GitHub.
              </p>
            </a>

            <a
              href="https://github.com/Clawdlinux/agentic-operator-core/tree/main/docs"
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-2xl p-6 transition-all duration-200 hover:-translate-y-1"
              style={cardStyle}
            >
              <BookOpen className="w-6 h-6 mb-4" style={{ color: currentTheme.accent.teal }} />
              <h3
                className="text-lg font-semibold mb-2"
                style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
              >
                Read the Docs
              </h3>
              <p style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}>
                Start with installation, architecture, and multi-tenancy guidance before deployment.
              </p>
            </a>
          </div>
        </div>
      </motion.div>
    </section>
  );
}
