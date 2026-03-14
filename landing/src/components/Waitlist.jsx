import { useState, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { CheckCircle, Loader2, Mail } from 'lucide-react';

const containerVariants = {
  hidden: { opacity: 0, y: 40 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.7, ease: 'easeOut', staggerChildren: 0.15 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 24 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.6, ease: 'easeOut' } },
};

const roles = [
  'Product Manager',
  'Growth / Marketing',
  'Sales / Revenue',
  'CTO / Founder',
  'VC / Investor',
  'Agency',
  'Other',
];

const inputStyle = {
  width: '100%',
  background: '#0d1525',
  border: '1px solid rgba(0,212,170,0.15)',
  borderRadius: '10px',
  padding: '12px 16px',
  color: '#fff',
  fontFamily: "'DM Sans', sans-serif",
  fontSize: '15px',
  outline: 'none',
  boxSizing: 'border-box',
  transition: 'border-color 0.2s, box-shadow 0.2s',
};

function FormInput({ label, ...props }) {
  const [focused, setFocused] = useState(false);
  return (
    <div style={{ marginBottom: 16 }}>
      <label
        style={{
          display: 'block',
          marginBottom: 6,
          fontSize: 13,
          fontFamily: "'DM Sans', sans-serif",
          color: 'rgba(255,255,255,0.5)',
          letterSpacing: '0.03em',
        }}
      >
        {label}
      </label>
      <input
        {...props}
        onFocus={(e) => { setFocused(true); props.onFocus && props.onFocus(e); }}
        onBlur={(e) => { setFocused(false); props.onBlur && props.onBlur(e); }}
        style={{
          ...inputStyle,
          borderColor: focused ? 'rgba(0,212,170,0.5)' : 'rgba(0,212,170,0.15)',
          boxShadow: focused ? '0 0 0 3px rgba(0,212,170,0.08)' : 'none',
        }}
      />
    </div>
  );
}

function FormSelect({ label, ...props }) {
  const [focused, setFocused] = useState(false);
  return (
    <div style={{ marginBottom: 16 }}>
      <label
        style={{
          display: 'block',
          marginBottom: 6,
          fontSize: 13,
          fontFamily: "'DM Sans', sans-serif",
          color: 'rgba(255,255,255,0.5)',
          letterSpacing: '0.03em',
        }}
      >
        {label}
      </label>
      <select
        {...props}
        onFocus={(e) => { setFocused(true); props.onFocus && props.onFocus(e); }}
        onBlur={(e) => { setFocused(false); props.onBlur && props.onBlur(e); }}
        style={{
          ...inputStyle,
          appearance: 'none',
          WebkitAppearance: 'none',
          backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%2300d4aa' d='M6 8L1 3h10z'/%3E%3C/svg%3E")`,
          backgroundRepeat: 'no-repeat',
          backgroundPosition: 'right 14px center',
          paddingRight: '36px',
          cursor: 'pointer',
          borderColor: focused ? 'rgba(0,212,170,0.5)' : 'rgba(0,212,170,0.15)',
          boxShadow: focused ? '0 0 0 3px rgba(0,212,170,0.08)' : 'none',
        }}
      >
        {props.children}
      </select>
    </div>
  );
}

export default function Waitlist() {
  const [form, setForm] = useState({ email: '', company: '', role: '' });
  const [status, setStatus] = useState('idle'); // idle | loading | success | error
  const [errorMsg, setErrorMsg] = useState('');

  // Client-side rate limiting: prevent rapid resubmission
  const lastSubmitRef = useRef(null);
  const COOLDOWN_MS = 30_000; // 30 seconds between submissions

  const SHEETS_URL =
    'https://script.google.com/macros/s/AKfycbwV1kA1LZbJOknuEogm6dNBNx8U1BU_djrC4lSKMzlPKmO0ARVCV6kD7MW0BWgGKsFJ/exec';

  const handleChange = (e) => {
    setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!form.email) return;

    // Client-side rate limit: 30s cooldown between submissions
    const now = Date.now();
    if (lastSubmitRef.current && now - lastSubmitRef.current < COOLDOWN_MS) {
      setErrorMsg('Please wait 30 seconds before submitting again.');
      setStatus('error');
      return;
    }
    lastSubmitRef.current = now;

    setStatus('loading');
    setErrorMsg('');

    try {
      // URLSearchParams sends application/x-www-form-urlencoded, which Apps Script parses into e.parameter
      // FormData (multipart) is NOT parsed by Apps Script — this is the correct encoding for no-cors + Apps Script
      const params = new URLSearchParams();
      params.append('email', form.email);
      params.append('company', form.company);
      params.append('role', form.role);
      params.append('timestamp', new Date().toISOString());

      // no-cors: Apps Script doesn't set CORS headers; we can't read the response — assume success
      await fetch(SHEETS_URL, { method: 'POST', mode: 'no-cors', body: params });

      setStatus('success');
    } catch (err) {
      console.error(err);
      setErrorMsg('Something went wrong. Please try again.');
      setStatus('error');
    }
  };

  return (
    <section
      id="waitlist"
      style={{ background: '#05080f' }}
      className="py-24 px-6 overflow-hidden relative"
    >
      {/* Decorative teal orb */}
      <div
        className="absolute pointer-events-none"
        style={{
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: 600,
          height: 600,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(0,212,170,0.07) 0%, transparent 70%)',
          filter: 'blur(40px)',
        }}
      />

      <motion.div
        className="max-w-xl mx-auto relative z-10"
        variants={containerVariants}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, amount: 0.2 }}
      >
        {/* Heading */}
        <motion.div variants={itemVariants} className="text-center mb-4">
          <span
            className="inline-block text-xs font-semibold tracking-widest uppercase mb-4 px-3 py-1 rounded-full"
            style={{
              color: '#00d4aa',
              background: 'rgba(0,212,170,0.08)',
              border: '1px solid rgba(0,212,170,0.2)',
              fontFamily: "'IBM Plex Mono', monospace",
            }}
          >
            Early Access
          </span>
          <h2
            className="text-4xl md:text-5xl font-bold text-white"
            style={{ fontFamily: "'Syne', sans-serif" }}
          >
            Join the Waitlist
          </h2>
        </motion.div>

        <motion.p
          variants={itemVariants}
          className="text-center text-lg mb-10"
          style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}
        >
          Get early access to automated competitive intelligence powered by AI.
        </motion.p>

        {/* Form card */}
        <motion.div
          variants={itemVariants}
          className="relative rounded-2xl p-8"
          style={{
            background: 'rgba(13,21,37,0.75)',
            border: '1px solid rgba(0,212,170,0.15)',
            backdropFilter: 'blur(16px)',
            boxShadow: '0 8px 60px rgba(0,0,0,0.5)',
          }}
        >
          {/* Gradient border accent */}
          <div
            className="absolute inset-x-0 top-0 h-px rounded-t-2xl"
            style={{
              background: 'linear-gradient(90deg, transparent, rgba(0,212,170,0.4), rgba(99,102,241,0.3), transparent)',
            }}
          />

          <AnimatePresence mode="wait">
            {status === 'success' ? (
              <motion.div
                key="success"
                initial={{ opacity: 0, scale: 0.85 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.4, ease: 'easeOut' }}
                className="flex flex-col items-center text-center py-8 gap-5"
              >
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ delay: 0.1, type: 'spring', stiffness: 200, damping: 14 }}
                >
                  <CheckCircle size={56} style={{ color: '#00d4aa' }} strokeWidth={1.5} />
                </motion.div>
                <div>
                  <h3
                    className="text-xl font-bold text-white mb-2"
                    style={{ fontFamily: "'Syne', sans-serif" }}
                  >
                    You're on the list!
                  </h3>
                  <p
                    className="text-base"
                    style={{ color: 'rgba(255,255,255,0.55)', fontFamily: "'DM Sans', sans-serif" }}
                  >
                    We'll reach out soon with your early access details.
                  </p>
                </div>
              </motion.div>
            ) : (
              <motion.form
                key="form"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                onSubmit={handleSubmit}
              >
                <FormInput
                  label="Email address *"
                  type="email"
                  name="email"
                  placeholder="you@company.com"
                  value={form.email}
                  onChange={handleChange}
                  required
                  autoComplete="email"
                />

                <FormInput
                  label="Company (optional)"
                  type="text"
                  name="company"
                  placeholder="Acme Corp"
                  value={form.company}
                  onChange={handleChange}
                  autoComplete="organization"
                />

                <FormSelect
                  label="Your role (optional)"
                  name="role"
                  value={form.role}
                  onChange={handleChange}
                >
                  <option value="" style={{ background: '#0d1525', color: '#fff' }}>
                    Select a role...
                  </option>
                  {roles.map((r) => (
                    <option key={r} value={r} style={{ background: '#0d1525', color: '#fff' }}>
                      {r}
                    </option>
                  ))}
                </FormSelect>

                {/* Error message */}
                {status === 'error' && (
                  <p
                    className="text-sm mb-4 text-center"
                    style={{ color: '#f87171', fontFamily: "'DM Sans', sans-serif" }}
                  >
                    {errorMsg}
                  </p>
                )}

                {/* Submit button */}
                <button
                  type="submit"
                  disabled={status === 'loading' || !form.email}
                  className="w-full flex items-center justify-center gap-2 py-3.5 rounded-xl font-semibold text-base transition-all duration-200 hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
                  style={{
                    background: 'linear-gradient(135deg, #00d4aa 0%, #00b894 100%)',
                    color: '#05080f',
                    fontFamily: "'DM Sans', sans-serif",
                    border: 'none',
                    cursor: status === 'loading' ? 'not-allowed' : 'pointer',
                    boxShadow: '0 4px 24px rgba(0,212,170,0.3)',
                    marginTop: 8,
                  }}
                >
                  {status === 'loading' ? (
                    <>
                      <Loader2 size={18} className="animate-spin" />
                      Joining...
                    </>
                  ) : (
                    <>
                      <Mail size={18} />
                      Request Early Access
                    </>
                  )}
                </button>
              </motion.form>
            )}
          </AnimatePresence>
        </motion.div>

        {/* Fine print */}
        <motion.p
          variants={itemVariants}
          className="text-center text-sm mt-6"
          style={{ color: 'rgba(255,255,255,0.3)', fontFamily: "'DM Sans', sans-serif" }}
        >
          No spam. Unsubscribe anytime. We'll reach out with early access.
        </motion.p>
      </motion.div>
    </section>
  );
}
