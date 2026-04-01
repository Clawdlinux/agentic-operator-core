import { useState } from 'react';
import { Github, X, Mail } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';

// ─── Legal modals content ────────────────────────────────────────────────────

const LEGAL = {
  terms: {
    title: 'Terms of Service',
    content: `Last updated: March 2026

1. Acceptance of Terms
By accessing clawdlinux.org or the Agentic Operator project materials, you agree to these terms. If you do not agree, do not use the site or distributed materials.

2. Open Source License
Agentic Operator core source code is licensed under the Apache License 2.0. Your use of the repository source code is governed by that license and the notices included in the project.

3. Website Content
Documentation, manifests, examples, and release notes are provided for informational purposes. Separate commercial offerings and managed support may be subject to separate agreements.

4. Acceptable Use
Do not use the project, examples, or website materials to violate law, abuse third-party services, or operate outside your organisation's authorization boundaries.

5. No Warranty
The project and site content are provided "as is", without warranty of any kind, express or implied, including merchantability, fitness for a particular purpose, or non-infringement.

6. Limitation of Liability
To the maximum extent permitted by law, Nine Rewards Solutions Pvt. Ltd. shall not be liable for any indirect, incidental, special, exemplary, or consequential damages arising from use of the site, project, or related materials.

7. Changes
We may update these terms from time to time. Continued use after publication of changes constitutes acceptance of the revised terms.

8. Contact
For questions about these terms, contact: 007ssancheti@gmail.com`,
  },
  privacy: {
    title: 'Privacy Policy',
    content: `Last updated: March 2026

1. Information We Collect
  This website provides a contact request form. When you submit it, we receive the details you provide (name, work email, company, and request message). If webhook delivery is unavailable, we may also receive information through email fallback.

2. How We Use Information
  We use inbound requests to:
• Respond to OSS and documentation questions
• Coordinate enterprise follow-up when requested
• Improve project guidance and support materials

3. Telemetry
This website does not knowingly collect personal telemetry from your cluster workloads. Project observability is configured by users inside their own environments.

4. Third-Party Services
Links to GitHub and other external services are governed by those services' own privacy policies.

5. Data Requests
You may request deletion of support correspondence by emailing: 007ssancheti@gmail.com

6. Contact
Nine Rewards Solutions Pvt. Ltd.
Email: 007ssancheti@gmail.com`,
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
If you experience or witness unacceptable behaviour, please report it to: 007ssancheti@gmail.com. All reports will be reviewed and investigated promptly and confidentially.

This Code of Conduct is adapted from the Contributor Covenant, version 2.1.`,
  },
};

// ─── Modal component ──────────────────────────────────────────────────────────

function LegalModal({ open, onClose, title, content, currentTheme, theme }) {
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
          background: currentTheme.bg.secondary,
          border: `1px solid ${currentTheme.border.medium}`,
          boxShadow:
            theme === 'dark'
              ? '0 0 60px rgba(0,212,170,0.08)'
              : '0 12px 42px rgba(15,23,42,0.16)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-6 py-4 flex-shrink-0"
          style={{ borderBottom: `1px solid ${currentTheme.border.light}` }}
        >
          <span
            className="text-base font-bold"
            style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
          >
            {title}
          </span>
          <button
            onClick={onClose}
            className="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
            style={{ color: currentTheme.text.tertiary, background: currentTheme.bg.tertiary }}
            onMouseEnter={(e) => { e.currentTarget.style.color = currentTheme.text.primary; }}
            onMouseLeave={(e) => { e.currentTarget.style.color = currentTheme.text.tertiary; }}
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
              color: currentTheme.text.tertiary,
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

function HexLogo({ accent }) {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
      <polygon points="16,2 29,9 29,23 16,30 3,23 3,9" stroke={accent} strokeWidth="1.5" fill={`${accent}1A`} />
      <polygon points="16,7 25,12 25,20 16,25 7,20 7,12" stroke={accent} strokeWidth="0.75" fill={`${accent}0D`} strokeDasharray="2 1" />
      <circle cx="16" cy="16" r="2" fill={accent} />
      <circle cx="16" cy="10" r="1" fill={accent} opacity="0.6" />
      <circle cx="21" cy="13" r="1" fill={accent} opacity="0.6" />
      <circle cx="21" cy="19" r="1" fill={accent} opacity="0.6" />
      <circle cx="16" cy="22" r="1" fill={accent} opacity="0.6" />
      <circle cx="11" cy="19" r="1" fill={accent} opacity="0.6" />
      <circle cx="11" cy="13" r="1" fill={accent} opacity="0.6" />
    </svg>
  );
}

// ─── Link helper ──────────────────────────────────────────────────────────────

function FooterLink({ href, external, onClick, children, baseColor, hoverColor }) {
  const common = {
    className: 'text-sm transition-colors duration-200 cursor-pointer',
    style: { color: baseColor, fontFamily: "'DM Sans', sans-serif", textDecoration: 'none' },
    onMouseEnter: (e) => { e.currentTarget.style.color = hoverColor; },
    onMouseLeave: (e) => { e.currentTarget.style.color = baseColor; },
  };
  if (onClick) return <button {...common} onClick={onClick} style={{ ...common.style, background: 'none', border: 'none', padding: 0 }}>{children}</button>;
  return <a href={href} target={external ? '_blank' : undefined} rel={external ? 'noopener noreferrer' : undefined} {...common}>{children}</a>;
}

// ─── Main footer ──────────────────────────────────────────────────────────────

export default function Footer() {
  const { currentTheme, theme } = useTheme();
  const [modal, setModal] = useState(null); // 'terms' | 'privacy' | 'conduct' | null

  const withAlpha = (hex, alpha) => `${hex}${alpha}`;

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
          currentTheme={currentTheme}
          theme={theme}
        />
      ))}

      <footer
        style={{
          background: currentTheme.bg.primary,
          borderTop: `1px solid ${withAlpha(currentTheme.accent.teal, '33')}`,
        }}
      >
        <div className="max-w-6xl mx-auto px-6 pt-16 pb-8">

          {/* Main grid: brand (2/5) + 3 columns */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-12 mb-16">

            {/* Brand column */}
            <div className="lg:col-span-2">
              <div className="flex items-center gap-3 mb-4">
                <HexLogo accent={currentTheme.accent.teal} />
                <span
                  className="text-lg font-bold"
                  style={{ fontFamily: "'Syne', sans-serif", color: currentTheme.text.primary }}
                >
                  Agentic Operator
                </span>
              </div>
              <p
                className="text-sm leading-relaxed max-w-xs mb-2"
                style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif" }}
              >
                Open-source Kubernetes operator for policy-aware AI agent workloads. Apache 2.0 licensed.
              </p>
              <p
                className="text-xs"
                style={{ color: currentTheme.text.muted, fontFamily: "'DM Sans', sans-serif" }}
              >
                Nine Rewards Solutions Pvt. Ltd. · Bangalore
              </p>
              <p
                className="text-xs mt-1"
                style={{ color: currentTheme.text.muted, fontFamily: "'DM Sans', sans-serif" }}
              >
                Agentic Operator · Apache 2.0 · Clawdlinux
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
                    background:
                      theme === 'dark'
                        ? withAlpha(currentTheme.bg.secondary, '8C')
                        : withAlpha(currentTheme.bg.secondary, 'D9'),
                    border: `1px solid ${currentTheme.border.light}`,
                    color: currentTheme.text.tertiary,
                    textDecoration: 'none',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.background = withAlpha(currentTheme.accent.teal, theme === 'dark' ? '1A' : '14');
                    e.currentTarget.style.borderColor = withAlpha(currentTheme.accent.teal, '4D');
                    e.currentTarget.style.color = currentTheme.accent.teal;
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.background =
                      theme === 'dark'
                        ? withAlpha(currentTheme.bg.secondary, '8C')
                        : withAlpha(currentTheme.bg.secondary, 'D9');
                    e.currentTarget.style.borderColor = currentTheme.border.light;
                    e.currentTarget.style.color = currentTheme.text.tertiary;
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
                style={{ color: currentTheme.text.muted, fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Product
              </h4>
              <ul className="space-y-3">
                {[
                  { label: 'Features', href: '#features' },
                  { label: 'Use Cases', href: '#use-cases' },
                  { label: 'Architecture', href: '#architecture' },
                  { label: 'Enterprise', href: '#products' },
                ].map(({ label, href }) => (
                  <li key={label}>
                    <FooterLink
                      href={href}
                      baseColor={currentTheme.text.tertiary}
                      hoverColor={currentTheme.accent.teal}
                    >
                      {label}
                    </FooterLink>
                  </li>
                ))}
              </ul>
            </div>

            {/* Resources column — real external links only */}
            <div>
              <h4
                className="text-xs font-semibold tracking-widest uppercase mb-5"
                style={{ color: currentTheme.text.muted, fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Resources
              </h4>
              <ul className="space-y-3">
                {[
                  { label: 'GitHub', href: 'https://github.com/Clawdlinux/agentic-operator-core', external: true },
                  { label: 'Documentation', href: 'https://github.com/Clawdlinux/agentic-operator-core/tree/main/docs', external: true },
                ].map(({ label, href, external }) => (
                  <li key={label}>
                    <FooterLink
                      href={href}
                      external={external}
                      baseColor={currentTheme.text.tertiary}
                      hoverColor={currentTheme.accent.teal}
                    >
                      {label}
                    </FooterLink>
                  </li>
                ))}
              </ul>
            </div>

            {/* Legal & Contact column */}
            <div>
              <h4
                className="text-xs font-semibold tracking-widest uppercase mb-5"
                style={{ color: currentTheme.text.muted, fontFamily: "'IBM Plex Mono', monospace", letterSpacing: '0.12em' }}
              >
                Legal
              </h4>
              <ul className="space-y-3">
                <li>
                  <FooterLink
                    onClick={() => openModal('terms')}
                    baseColor={currentTheme.text.tertiary}
                    hoverColor={currentTheme.accent.teal}
                  >
                    Terms of Service
                  </FooterLink>
                </li>
                <li>
                  <FooterLink
                    onClick={() => openModal('privacy')}
                    baseColor={currentTheme.text.tertiary}
                    hoverColor={currentTheme.accent.teal}
                  >
                    Privacy Policy
                  </FooterLink>
                </li>
                <li>
                  <FooterLink
                    onClick={() => openModal('conduct')}
                    baseColor={currentTheme.text.tertiary}
                    hoverColor={currentTheme.accent.teal}
                  >
                    Code of Conduct
                  </FooterLink>
                </li>
                <li>
                  <a
                    href="mailto:007ssancheti@gmail.com"
                    className="text-sm transition-colors duration-200"
                    style={{ color: currentTheme.text.tertiary, fontFamily: "'DM Sans', sans-serif", textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '6px' }}
                    onMouseEnter={(e) => { e.currentTarget.style.color = currentTheme.accent.teal; }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = currentTheme.text.tertiary; }}
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
            style={{ borderTop: `1px solid ${currentTheme.border.light}` }}
          >
            <p
              className="text-xs text-center sm:text-left"
              style={{ color: currentTheme.text.muted, fontFamily: "'DM Sans', sans-serif" }}
            >
              &copy; {new Date().getFullYear()} Nine Rewards Solutions Pvt. Ltd. · Bangalore · Apache 2.0
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
                    color: currentTheme.text.muted,
                    fontFamily: "'DM Sans', sans-serif",
                    background: 'none',
                    border: 'none',
                    cursor: 'pointer',
                    padding: 0,
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.color = currentTheme.text.tertiary; }}
                  onMouseLeave={(e) => { e.currentTarget.style.color = currentTheme.text.muted; }}
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
