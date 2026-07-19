#!/usr/bin/env node

const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

function fail(message) {
  process.stderr.write(`[FAIL] ${message}\n`);
  process.exit(1);
}

const [url, outputPath, timeoutValue, chromePath] = process.argv.slice(2);
if (!url || !outputPath || !timeoutValue || !chromePath) {
  fail('usage: record-dashboard.js URL OUTPUT.webm TIMEOUT_SECONDS CHROME_PATH');
}
if (!outputPath.endsWith('.webm')) {
  fail('output path must end in .webm');
}
const timeoutSeconds = Number(timeoutValue);
if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 20 || timeoutSeconds > 180) {
  fail('timeout must be an integer from 20 to 180');
}
if (!fs.existsSync(chromePath)) {
  fail(`Chrome executable not found: ${chromePath}`);
}

const outputDirectory = path.dirname(outputPath);
const temporaryDirectory = fs.mkdtempSync(path.join(outputDirectory, '.playwright-video-'));

(async () => {
  let browser;
  try {
    browser = await chromium.launch({
      headless: true,
      executablePath: chromePath,
    });
    const context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      recordVideo: {
        dir: temporaryDirectory,
        size: { width: 1920, height: 1080 },
      },
      colorScheme: 'dark',
      reducedMotion: 'no-preference',
    });
    const page = await context.newPage();
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 10_000 });
    await page.locator('#sourceState').waitFor({ state: 'visible', timeout: 10_000 });
    const video = page.video();
    if (!video) {
      throw new Error('Playwright did not create a video stream');
    }

    await page.waitForFunction(() => {
      const sourceState = document.querySelector('#sourceState')?.textContent;
      const auditVerdict = document.querySelector('#auditVerdict')?.textContent;
      const stage = document.querySelector('#stageLine')?.textContent;
      return sourceState === 'complete'
        && auditVerdict === 'TAMPERED COPY REJECTED'
        && stage === 'Live run complete.';
    }, null, { timeout: timeoutSeconds * 1000 });
    await page.waitForTimeout(2_000);

    await context.close();
    await video.saveAs(outputPath);
    await browser.close();
    browser = null;
    fs.chmodSync(outputPath, 0o600);
    process.stdout.write(`${outputPath}\n`);
  } catch (error) {
    if (browser) {
      await browser.close().catch(() => {});
    }
    throw error;
  } finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  }
})().catch((error) => {
  fail(error.stack || String(error));
});
