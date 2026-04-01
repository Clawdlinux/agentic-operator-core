# Google Sheets Contact Flow Setup

The landing contact form in `src/components/Waitlist.jsx` can submit to a Google Apps Script webhook.

## 1. Create Google Sheet

Create columns in row 1:

- `submittedAt`
- `name`
- `email`
- `company`
- `message`
- `source`
- `page`

## 2. Deploy Apps Script webhook

Use a basic `doPost(e)` handler that reads URL-encoded params and appends to the sheet.

Required fields expected by landing:

- `name`
- `email`
- `company`
- `message`
- `source`
- `submittedAt`
- `page`

## 3. Configure env vars

Local:

```bash
cd landing
cp .env.example .env
# set VITE_CONTACT_FORM_URL to your Apps Script web app URL
```

Vercel production:

```bash
cd landing
npx vercel env add VITE_CONTACT_FORM_URL production
npx vercel env add VITE_CONTACT_FALLBACK_EMAIL production
npx vercel --prod --yes
```

## 4. Verify submission

1. Run `npm run dev`.
2. Submit the contact form.
3. Confirm a new row in Google Sheets.

If webhook delivery fails, the UI falls back to a prefilled mailto flow.
