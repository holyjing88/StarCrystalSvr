import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';

/** serve assets/h5 so ../js/crystabox-ad.js resolves under same origin+port */
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const gamePath = '/parkinglot_game1/index.html';

const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml; charset=utf-8',
  '.png': 'image/png',
};

const server = http.createServer((req, res) => {
  const urlPath = decodeURIComponent((req.url || '/').split('?')[0]);
  if (urlPath === '/favicon.ico') {
    res.writeHead(204);
    res.end();
    return;
  }
  const safe = path.normalize(urlPath).replace(/^(\.\.[\/\\])+/, '');
  const rel = safe === '/' || safe === '\\' ? gamePath.slice(1) : safe.replace(/^[\/\\]+/, '');
  const file = path.join(root, rel);
  if (!file.startsWith(root)) {
    res.writeHead(403);
    res.end();
    return;
  }
  fs.readFile(file, (err, data) => {
    if (err) {
      res.writeHead(404);
      res.end('404');
      return;
    }
    const ext = path.extname(file);
    res.writeHead(200, { 'Content-Type': mime[ext] || 'application/octet-stream' });
    res.end(data);
  });
});

await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const port = server.address().port;
const base = `http://127.0.0.1:${port}/`;

const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
const errors = [];
const notFound = [];
page.on('console', (msg) => {
  const t = msg.text();
  if (msg.type() === 'error') errors.push(`console: ${t}`);
});
page.on('pageerror', (err) => errors.push(`pageerror: ${err.message}`));
page.on('response', (res) => {
  if (res.status() === 404) notFound.push(res.url());
});

await page.goto(base.slice(0, -1) + gamePath, { waitUntil: 'domcontentloaded', timeout: 15000 });
await page.setViewportSize({ width: 420, height: 900 });
await page.waitForSelector('.arrowBlock', { timeout: 10000 });

let rounds = 0;
while (rounds < 28) {
  const n = await page.locator('.arrowBlock').count();
  if (!n) break;
  await page.locator('.arrowBlock').first().click({ timeout: 1500 }).catch(() => {});
  await page.waitForTimeout(380);
  rounds++;
}

await page.waitForTimeout(600);
await browser.close();
server.close();

if (notFound.length) {
  console.error('404 URLs:\n' + notFound.join('\n'));
}
if (errors.length) {
  console.error('FAIL\n' + errors.join('\n'));
  process.exit(1);
}
console.log(`OK (${rounds} taps, stress)`);
process.exit(0);
