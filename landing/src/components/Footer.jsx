import { useState } from 'react';
import { Github, X, Mail } from 'lucide-react';

// ─── Legal modals content ────────────────────────────────────────────────────

const LEGAL = {
  terms: {
    title: 'Terms of Service',
    content: `Last updated: March 2026

1. Acceptance of Terms
By accessing or using Agentic Operator ("the Software"), you agree to be bound by these Terms of Service. If you do not agree, do not use the Software.

2. License
Agentic Operator is released under the Apache License 2.0. You are free to use, modify, and distribute the Software in accordance with the terms of that license.

3. Use of Service
You agree to use the Software only for lawful purposes and in a way that does not infringe the rights of others. You must not use the Software to deploy agents that perform unauthorized actions, access systems without permission, or violate applicable laws.

4. No Warranty
The Software is provided "as is", without warranty of any kind, express or implied. Nine Rewards Solutions Private Limited makes no warranties regarding fitness for a particular purpose, merchantability, or non-infringement.

5. Limitation of Liability
To the maximum extent permitted by law, Nine Rewards Solutions Private Limited shall not be liable for any indirect, incidental, special, exemplary, or consequential damages arising from your use of the Software.

6. Changes
We reserve the right to update these terms at any time. Continued use of the Software after changes constitutes acceptance of the new terms.

7. Contact
For questions about these terms, contact: shreyanshsancheti09@gmail.com`,
  },
  privacy: {
    title: 'Privacy Policy',
    content: `Last updated: March 2026

1. Information We Collect
When you join our waitlist, we collect your email address and optionally your company name and role. We do not collect any other personal information automatically.

2. How We Use Your Information
We use your email solely to:
• Send updates about Agentic Operator releases and early access invitations
• Respond to your inquiries

3. Data Storage
Waitlist submissions are stored in Google Sheets, accessible only to Nine Rewards Solutions Private Limited team members.

4. No Third-Party Sharing
We do not sell, trade, or transfer your personal information to third parties. We do not use your data for advertising purposes.

5. Open Source Telemetry
The Agentic Operator software itself does not collect any telemetry or send data to our servers. All agent workloads run entirely within your Kubernetes cluster.

6. Your Rights
You may request deletion of your data at any time by emailing: shreyanshsancheti09@gmail.com

7. Cookies
This website does not use cookies beyond what is strictly necessary for functionality.

8. Contact
Nine Rewards Solutions Private Limited
Email: shreyanshsancheti09@gmail.com`,
  },
  conduct: {
    title: 'Code of Conduct',
    content: `We are committed to providing a welcoming, inclusive, and harassment-free community for everyone, regardless of experience level, gender, gender identity, sexual orientation, disability, personal appearance, race, ethnicity, age, religion, or nationality.

Expected Behaviour
• Be respectful and constructive in all interactions
• Welcome newcomers and help them get started
• Give credit where credit is due
• Focus on what is best for the community and the project
• Show empathy toward other community members

Unacceptable Behaviour
• Harassment, intimidation, or discrimination in any form
• Derogatory comments or personal attacks
• Publishing others' private information without consent
• Trolling, insulting, or demeaning comments
• Sustained disruption of discussions or events

Enforcement
Violations may result in a warning, temporary ban, or permanent exclusion from community spaces, at the discretion of the maintainers.

Reporting
If you experience or witness unacceptable behaviour, please report it to: shreyanshsancheti09@gmail.com. All reports will be reviewed and investigated promptly and confidentially.

This Code of Conduct is adapted from the Contributor Covenant, version 2.1.`,
  },
};

// ─── Modal component ──────────────────────────────────────────────────────────

function LegalModal({ open, onClose, title, content }) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(8px)' }}
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-lg max-h-[80vh] flex flex-col rounded-2xl overflow-hidden"
        style={{
          background: '#0d1525',
          border: '1px solid rgba(0,212,170,0.2)',
          boxShadow: '0 0 60px rgba(0,212,170,0.08)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-6 py-4 flex-shrink-0"
          style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}
        >
          <span
            className="text-base font-bold"
            style={{ fontFamily: "'Syne', sans-serif", color: '#e2e8f0' }}
          >
            {title}
          </span>
          <button
            onClick={onClose}
            className="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
            style={{ color: 'rgba(255,255,255,0.4)', background: 'rgba(255,255,255,0.04)' }}
            onMouseEnter={(e) => { e.currentTarget.style.color = '#e2e8f0'; }}
            onMouseLeave={(e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.4)'; }}
          >
            <X size={16} />
          </button>
        </div>
        {/* Content */}
        <div className="overflow-y-auto px-6 py-5 flex-1">
          <pre
            className="text-sm leading-relaxed whitespace-pre-wrap"
            style={{
              fontFamily: "'DM Sans', sans-serif",
              color: 'rgba(255,255,255,0.55)',
            }}
          >
            {content}
          </pre>
        </div>
      </div>
    </div>
  );
}

// ─── Hexagon logo ─────────────────────────────────────────────────────────────

function HexLogo() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
      <polygon points="16,2 29,9 29,23 16,30 3,23 3,9" stroke="#00d4aa" strokeWidth="1.5" fill="rgba(0,212,170,0.1)" />
      <polygon points="16,7 25,12 25,20 16,25 7,20 7,12" stroke="#00d4aa" strokeWidth="0.75" fill="rgba(0,212,170,0.05)" strokeDasharray="2 1" />
      <circle cx="16" cy="16" r="2" fill="#00d4aa" />
      <circle cx="16" cy="10" r="1" fill="#00d4aa" opacity="0.6" />
      <circle cx="21" cy="13" r="1" fill="#00d4aa" opacity="0.6" />
      <circle cx="21" cy="19" r="1" fill="#00d4aa" opacity="0.6" />
      <circle cx="16" cy="22" r="1" fill="#00d4aa" opacity="0.6" />
      <circle cx="11" cy="19" r="1" fill="#00d4aa" opacity="0.6" />
      <circle cx="11" cy="13" r="1" fill="#00d4aa" opacity="0.6" />
    </svg>
  );
}

// ─── Link helper ──────────────────────────────────────────────────────────────

function FooterLink({ href, external, onClick, children }) {
  const common = {
    className: 'text-sm transition-colors duration-200 cursor-pointer',
    style: { color: 'rgba(255,255,255,0.45)', fontFamily: "'DM Sans', sans-serif", textDecoration: 'none' },
    onMouseEnter: (e) => { e.currentTarget.style.color = '#00d4aa'; },
    onMouseLeave: (e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.45)'; },
  };
  if (onClick) return <button {...common} onClick={onClick} style={{ ...common.style, background: 'none', border: 'none', padding: 0 }}>{children}</button>;
  return <a href={href} target={external ? '_blank' : undefined} rel={external ? 'noopener noreferrer' : undefined} {...common}>{children}</a>;
}

// ─── Main footer ──────────────────────────────────────────────────────────────

export default function Footer() {
  const [modal, setModal] = useState(null); // 'terms' | 'privacy' | 'conduct' | null

  const openModal = (key) => setModal(key);
  const closeModal = () => setModal(null);

  return (
    <>
      {/* Legal modals */}
      {['terms', 'privacy', 'conduct'].map((key) => (
        <LegalModal
          key={key}
          open={modal === key}
          onClose={closeModal}
          title={LEGAL[key].title}
          content={LEGAL[key].content}
        />
      ))}

      <footer style={{ background: '#05080f', borderTop: '1px solid rgba(0,212,170,0.2)' }}>
        <div className="max-w-6xl mx-auto px-6 pt-16 pb-8">

          {/* Main grid: brand (2/5) + 3 columns */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-12 mb-16">

            {/* Brand column */}
            <div className="lg:col-span-2">
              <div className="flex items-center gap-3 mb-4">
                <HexLogo />
                <span className="text-lg font-bold text-white" style={{ fontFamily: "'Syne', sans-serif" }}>
                  Agentic Operator
                </span>
              </div>
              <p
                className="text-sm leading-relaxed max-w-xs mb-2"
                style={{ color: 'rgba(255,255,255,0.4)', fontFamily: "'DM Sans', sans-serif" }}
              >
                Deploy AI agents in your Kubernetes cluster. Production-grade orchestration for the agentic era.
              </p>
              <p
                className="text-xs"
                style={{ color: 'rgba(255,255,255,0.22)', fontFamily: "'DM Sans', sans-serif" }}
              >
                A product of Nine Rewards Solutions Private Limited
              </p>

              {/* GitHub only */}
              <div className="flex items-center gap-3 mt-6">
                <a
                  href="https://github.com/Clawdlinux/agentic-operator-core"
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label="GitHub"
                  className="flex items-center justify-center w-9 h-9 rounded-lg transition-all duration-200 hover:scale-110"
                  style={{
                    background: 'rgba(255,255,255,0.05)',
                    border: '1px solid rgba(255,255,255,0.08)',
                    color: 'rgba(255,255,255,0.5)',
                    textDecoration: 'none',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.background = 'rgba(0,212,170,0.1)';
                    e.currentTarget.style.borderColor = 'rgba(0,212,170,0.3)';
                    e.currentTarget.style.color = '#00d4aa';
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.background = 'rgba(255,255,255,0.05)';
                    e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)';
                    e.currentTarget.style.color = 'rgba(255,255,255,0.5)';
                  }}
                >
                  <Github size={17} />
                </a>
              </div>
            </div>

            {/* Product column — real in-page anchors only */}
            <div>
              <h4
                className="text-xs font-semibold tracking-widest uppercase mb-5"
                style={{ color: 'rgba(255,255,255,0.35)', fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Product
              </h4>
              <ul className="space-y-3">
                {[
                  { label: 'Features', href: '#features' },
                  { label: 'Use Cases', href: '#use-cases' },
                  { label: 'Architecture', href: '#architecture' },
                  { label: 'Join Waitlist', href: '#waitlist' },
                ].map(({ label, href }) => (
                  <li key={label}><FooterLink href={href}>{label}</FooterLink></li>
                ))}
              </ul>
            </div>

            {/* Resources column — real external links only */}
            <div>
              <h4
                className="text-xs font-semibold tracking-widest uppercase mb-5"
                style={{ color: 'rgba(255,255,255,0.35)', fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Resources
              </h4>
              <ul className="space-y-3">
                {[
                  { label: 'GitHub', href: 'https://github.com/Clawdlinux/agentic-operator-core', external: true },
                  { label: 'Helm Chart', href: 'https://registry.digitalocean.com/agentic-operator/charts/agentic-operator', external: true },
                ].map(({ label, href, external }) => (
                  <li key={label}><FooterLink href={href} external={external}>{label}</FooterLink></li>
                ))}
              </ul>
            </div>

            {/* Legal & Contact column */}
            <div>
              <h4
                className="text-xs font-semibold tracking-widest uppercase mb-5"
                style={{ color: 'rgba(255,255,255,0.35)', fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Legal
              </h4>
              <ul className="space-y-3">
                <li><FooterLink onClick={() => openModal('terms')}>Terms of Service</FooterLink></li>
                <li><FooterLink onClick={() => openModal('privacy')}>Privacy Policy</FooterLink></li>
                <li><FooterLink onClick={() => openModal('conduct')}>Code of Conduct</FooterLink></li>
                <li>
                  <a
                    href="mailto:shreyanshsancheti09@gmail.com"
                    className="text-sm transition-colors duration-200"
                    style={{ color: 'rgba(255,255,255,0.45)', fontFamily: "'DM Sans', sans-serif", textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '6px' }}
                    onMouseEnter={(e) => { e.currentTarget.style.color = '#00d4aa'; }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.45)'; }}
                  >
                    <Mail size={13} />
                    Contact
                  </a>
                </li>
              </ul>
            </div>

          </div>

          {/* Bottom bar */}
          <div
            className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-8"
            style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}
          >
            <p
              className="text-xs text-center sm:text-left"
              style={{ color: 'rgba(255,255,255,0.25)', fontFamily: "'DM Sans', sans-serif" }}
            >
              &copy; {new Date().getFullYear()} Nine Rewards Solutions Private Limited &middot; Apache 2.0 License
            </p>
            <div className="flex items-center gap-5">
              {[
                { label: 'Terms', key: 'terms' },
                { label: 'Privacy', key: 'privacy' },
                { label: 'Code of Conduct', key: 'conduct' },
              ].map(({ label, key }) => (
                <button
                  key={key}
                  onClick={() => openModal(key)}
                  className="text-xs transition-colors duration-200"
                  style={{
                    color: 'rgba(255,255,255,0.2)',
                    fontFamily: "'DM Sans', sans-serif",
                    background: 'none',
                    border: 'none',
                    cursor: 'pointer',
                    padding: 0,
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.5)'; }}
                  onMouseLeave={(e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.2)'; }}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

        </div>
      </footer>
    </>
  );
}
