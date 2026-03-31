# Agentic Operator Landing

Production React + Vite landing page for the open-core Agentic Operator project.

## What this landing does

- Positions Agentic Operator as a Kubernetes-native multi-agent operator.
- Routes visitors into the open-source contribution path instead of a marketing waitlist.
- Links directly to the repo, docs, and pull request workflow.

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
- `src/components/Waitlist.jsx`: contribution CTA section replacing the old waitlist flow
- `src/components/Footer.jsx`: OSS/privacy messaging
- `index.html`: SEO and social metadata
- `vercel.json`: response headers and cache policy
