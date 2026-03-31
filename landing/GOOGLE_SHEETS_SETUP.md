# Google Sheets Waitlist Setup

This document is retained only as a legacy reference.

The current open-core landing page no longer runs a Google Sheets waitlist flow. The `landing/src/components/Waitlist.jsx` section is now a contribution CTA that sends users to GitHub and docs instead of posting to Apps Script.

## If a waitlist flow is reintroduced later

Use Vercel for deployment.

1. Create the target Google Sheet and Apps Script webhook.
2. Configure the webhook URL through a Vite env var such as `VITE_WAITLIST_WEBHOOK_URL`.
3. Set the production value in Vercel:

```bash
cd landing
npx vercel env add VITE_WAITLIST_WEBHOOK_URL production
npx vercel --prod --yes
```

## Operational note

If a future form uses `mode: 'no-cors'`, browser-side code will not receive Apps Script error payloads. Treat Google Sheets as the source of truth for submission success.
