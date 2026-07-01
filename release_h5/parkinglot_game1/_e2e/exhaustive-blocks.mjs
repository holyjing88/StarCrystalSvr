/**
 * 穷尽测试：空闲时轮换点击每一块，直到清空或判定卡住。
 */
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
/** e2e_exhaustive=1：满车位 + 选关 1，避免默认 2 车位下穷举卡死 */
const gamePath = '/parkinglot_game1/index.html?e2e_exhaustive=1';

const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml; charset=utf-8',
  '.wav': 'audio/wav',
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
const url = `http://127.0.0.1:${port}${gamePath}`;

const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
const errors = [];
page.on('console', (msg) => {
  if (msg.type() === 'error') errors.push(`console: ${msg.text()}`);
});
page.on('pageerror', (err) => errors.push(`pageerror: ${err.message}`));

await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 20000 });
await page.setViewportSize({ width: 420, height: 900 });
await page.waitForSelector('#playfield .arrowBlock', { timeout: 12000 });

async function waitPlayfieldIdle(timeoutMs) {
  await page.waitForFunction(
    () => !document.querySelector('#playfield .arrowBlock.parkingMove'),
    { timeout: timeoutMs }
  );
}

async function blockCount() {
  return page.locator('#playfield .arrowBlock').count();
}

/** 车位占满或射击暂停时需点「等待中的车位」才会继续消格、腾出空位，否则穷举点箭头会假卡死 */
async function maybeTapWaitingParkBay() {
  const bays = page.locator('.parkBay-occupied.parkBay-waitTap');
  if ((await bays.count()) === 0) return;
  await bays.first().click({ timeout: 3000 }).catch(() => {});
  await page.waitForTimeout(120);
}

await waitPlayfieldIdle(8000);

let rounds = 0;
let noEscapeStreak = 0;
const maxRounds = 500;
const startCount = await blockCount();

while ((await blockCount()) > 0 && rounds < maxRounds) {
  await waitPlayfieldIdle(40000);
  const n = await blockCount();
  if (n === 0) break;

  await maybeTapWaitingParkBay();
  await waitPlayfieldIdle(15000);

  const idx = rounds % n;
  await page.locator('#playfield .arrowBlock').nth(idx).click({ timeout: 5000 });
  rounds++;

  await waitPlayfieldIdle(40000);
  const after = await blockCount();

  if (after >= n) {
    noEscapeStreak++;
    if (noEscapeStreak > Math.max(24, n * 10)) {
      const left = await blockCount();
      await browser.close();
      server.close();
      console.error(`FAIL: stuck (blocks=${left}, rounds=${rounds})`);
      if (errors.length) console.error(errors.join('\n'));
      process.exit(1);
    }
  } else {
    noEscapeStreak = 0;
  }
}

const finalLeft = await blockCount();
await browser.close();
server.close();

if (finalLeft > 0) {
  console.error(`FAIL: ${finalLeft} blocks left after ${rounds} taps (started ${startCount})`);
  if (errors.length) console.error(errors.join('\n'));
  process.exit(1);
}

if (rounds >= maxRounds) {
  console.error(`FAIL: max rounds ${maxRounds}`);
  process.exit(1);
}

if (errors.length) {
  console.error('FAIL console:\n' + errors.join('\n'));
  process.exit(1);
}

console.log(`OK exhaustive: ${startCount} -> 0 in ${rounds} taps`);
