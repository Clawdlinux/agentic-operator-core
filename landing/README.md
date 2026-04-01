# Agentic Operator Landing

Production React + Vite landing page for the open-core Agentic Operator project.

## What this landing does

- Positions Agentic Operator as a Kubernetes-native multi-agent operator.
- Routes visitors into a working contact request flow plus open-source contribution paths.
- Links directly to the repo, docs, and pull request workflow.

## Contact form destination

The contact section uses:

- `VITE_CONTACT_FORM_URL` for webhook delivery (`application/x-www-form-urlencoded`).
- `VITE_CONTACT_FALLBACK_EMAIL` for mailto fallback if webhook delivery is not configured.

Example local setup:

```bash
cp .env.example .env
# set VITE_CONTACT_FORM_URL to your webhook endpoint
```

## Local development

```bash
npm install
npm run dev
```

Default local URL: http://127.0.0.1:5173

## Quality gate

```bash
npm run lint
npm run build
```

Both commands should pass before deployment.

## Production deploy (Vercel)

```bash
npx vercel --prod --yes
```

Security and caching headers are managed in `vercel.json`.

## Important files

- `src/App.jsx`: page composition and section ordering
- `src/components/Waitlist.jsx`: contact form section with webhook + email fallback
- `src/components/Footer.jsx`: OSS/privacy messaging
- `index.html`: SEO and social metadata
- `vercel.json`: response headers and cache policy
