# Google Sheets Waitlist Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the live Google Apps Script URL into the landing page waitlist form so submissions are captured in Google Sheets.

**Architecture:** Replace `import.meta.env.VITE_GOOGLE_SHEETS_URL` env-var read with a hardcoded constant in `Waitlist.jsx`. The form already uses `mode: 'no-cors'` FormData POST — the only missing piece is the URL. After wiring, do a local build to confirm zero errors, then commit, push, create PR, review, merge, and redeploy.

**Tech Stack:** React, Vite, Framer Motion, Vercel, Google Apps Script web app

---

### Task 1: Wire the Apps Script URL into Waitlist.jsx

**Files:**
- Modify: `landing/src/components/Waitlist.jsx` — replace env-var read with constant

**Step 1: Replace env-var line with hardcoded constant**

In `Waitlist.jsx`, change line 115 from:
```js
const sheetsUrl = import.meta.env.VITE_GOOGLE_SHEETS_URL;
```
to:
```js
const SHEETS_URL = 'https://script.google.com/macros/s/AKfycbwV1kA1LZbJOknuEogm6dNBNx8U1BU_djrC4lSKMzlPKmO0ARVCV6kD7MW0BWgGKsFJ/exec';
```

Also update `handleSubmit` to use `SHEETS_URL` instead of `sheetsUrl`, and remove the `if (sheetsUrl)` guard (it's always defined now):

```js
const handleSubmit = async (e) => {
  e.preventDefault();
  if (!form.email) return;

  setStatus('loading');
  setErrorMsg('');

  try {
    const data = new FormData();
    data.append('email', form.email);
    data.append('company', form.company);
    data.append('role', form.role);
    data.append('timestamp', new Date().toISOString());

    await fetch(SHEETS_URL, { method: 'POST', mode: 'no-cors', body: data });

    setStatus('success');
  } catch (err) {
    console.error(err);
    setErrorMsg('Something went wrong. Please try again.');
    setStatus('error');
  }
};
```

**Step 2: Verify build passes**

```bash
cd /Users/sunny/.openclaw/workspace/agentic-k8s-operator/landing
npm run build 2>&1 | tail -20
```
Expected: `✓ built in X.XXs` — no errors.

**Step 3: Commit (signed)**

```bash
cd /Users/sunny/.openclaw/workspace/agentic-k8s-operator
git add landing/src/components/Waitlist.jsx
git commit -s -m "feat: wire google apps script url to waitlist form"
```

---

### Task 2: Push and create PR

**Step 1: Push branch**

```bash
cd /Users/sunny/.openclaw/workspace/agentic-k8s-operator
git push origin feat/landing-page
```

**Step 2: Create PR**

```bash
gh pr create \
  --title "feat: wire Google Sheets waitlist integration" \
  --body "## Summary
- Hardcodes the Google Apps Script web app URL directly in Waitlist.jsx
- Removes the \`VITE_GOOGLE_SHEETS_URL\` env-var dependency (URL is a public webhook, not a secret)
- Adds \`timestamp\` field to every submission for easier tracking in Sheets
- No behaviour changes to the form UI

## Test plan
- [ ] Submit the waitlist form on staging/production
- [ ] Confirm entry appears in Google Sheets
- [ ] Confirm success animation shows after submit" \
  --base main \
  --head feat/landing-page
```

---

### Task 3: Review and merge PR

**Step 1: Review with the code-review skill (or inline)**

Check diff:
```bash
gh pr diff
```
Verify: only `Waitlist.jsx` changed, URL is correct, no secrets exposed.

**Step 2: Merge PR**

```bash
gh pr merge --squash --delete-branch
```

---

### Task 4: Deploy to Vercel

**Step 1: Deploy from main**

```bash
cd /Users/sunny/.openclaw/workspace/agentic-k8s-operator/landing
git pull  # ensure on main with merge commit
npx vercel --prod --yes
```

**Step 2: Smoke-test live form**

Navigate to the Vercel production URL for the landing app and verify the form renders and the submit button is enabled.
