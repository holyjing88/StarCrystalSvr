/**
 * 自检：占领车位存在时，验证 pivot 底边中心的几何算法与 rotate(0) 下包围盒底边中心接近。
 * 无占领车位则跳过（exit 0）。
 */
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const gamePath = '/parkinglot_game1/index.html';

const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript',
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
const url = `http://127.0.0.1:${port}${gamePath}`;

const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 20000 });
await page.setViewportSize({ width: 420, height: 900 });
await page.waitForSelector('.arrowBlock', { timeout: 12000 });

/** 与 smoke.mjs 同档点击，尽量刷出带炮塔占领车位 */
let rounds = 0;
while (rounds < 32) {
  const has = await page.locator('.parkBay-occupied .parkTurretBarrelPivot').count();
  if (has) break;
  const n = await page.locator('.arrowBlock').count();
  if (!n) break;
  await page.locator('.arrowBlock').first().click({ timeout: 1500 }).catch(() => {});
  await page.waitForTimeout(380);
  rounds++;
}
await page.waitForTimeout(500);

const result = await page.evaluate(() => {
  var mount = document.querySelector('.parkBay-occupied .parkTurretMount');
  var pivot = document.querySelector('.parkBay-occupied .parkTurretBarrelPivot');
  if (!mount || !pivot) return { skip: true };
  pivot.style.transform = 'rotate(0deg)';
  var mr = mount.getBoundingClientRect();
  var bx = 0;
  var by = 0;
  var el = pivot;
  while (el && el !== mount) {
    bx += el.offsetLeft;
    by += el.offsetTop;
    el = el.offsetParent;
  }
  var pw = pivot.offsetWidth;
  var ph = pivot.offsetHeight;
  var px;
  var py;
  if (el !== mount) {
    var pivotBottomPx = parseFloat(window.getComputedStyle(pivot).bottom);
    if (isNaN(pivotBottomPx)) pivotBottomPx = 12;
    px = mr.left + mr.width * 0.5;
    py = mr.bottom - pivotBottomPx;
  } else {
    px = mr.left + bx + pw * 0.5;
    py = mr.top + by + ph;
  }
  var pr = pivot.getBoundingClientRect();
  var ox = (pr.left + pr.right) * 0.5;
  var oy = pr.bottom;
  var errPx = Math.hypot(px - ox, py - oy);
  return { skip: false, errPx };
});

await browser.close();
server.close();

if (result.skip) {
  console.log('SKIP turret-pivot-check (no occupied bay with turret)');
  process.exit(0);
}

if (result.errPx > 4) {
  console.error(`FAIL turret pivot delta ${result.errPx.toFixed(2)}px (expect <= 4)`);
  process.exit(1);
}

console.log(`OK turret-pivot-check delta ${result.errPx.toFixed(2)}px`);
