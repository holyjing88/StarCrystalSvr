/**
 * 复现并守卫「多车位同时排队瞄准」竞态：连续调用 aim + 其间 renderTarget 清空目标格，
 * 旧闭包 cell 会失效；修复后应在 rAF 内重新 query，炮口仍对准各自目标。
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
const url = `http://127.0.0.1:${port}${gamePath}?e2e_turret=1`;

const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 20000 });
await page.setViewportSize({ width: 420, height: 900 });
await page.waitForFunction(() => window.__parkinglotE2e && window.__parkinglotE2e.aimParkTurretThenShoot, {
  timeout: 15000,
});

const sched = await page.evaluate(() => {
  var E = window.__parkinglotE2e;
  var cells = Array.from(document.querySelectorAll('.targetGrid .tgCell:not(.empty)')).slice(0, 8);
  if (cells.length < 8) return { err: 'need 8 colored tgCells got ' + cells.length };
  var picks = [];
  var i;
  for (i = 0; i < 8; i++) {
    picks.push({ tr: +cells[i].dataset.r, tc: +cells[i].dataset.c });
  }
  for (i = 0; i < 8; i++) {
    var p = picks[i];
    E.aimParkTurretThenShoot(i, 1, p.tr, p.tc, 0, function () {});
  }
  E.renderTarget();
  return { picks };
});

if (sched.err) {
  console.error('FAIL schedule:', sched.err);
  await browser.close();
  server.close();
  process.exit(1);
}

await page.waitForTimeout(320);

const bad = await page.evaluate((picks) => {
  function wrapDeg(d) {
    d = d % 360;
    if (d > 180) d -= 360;
    if (d <= -180) d += 360;
    return d;
  }
  var out = [];
  var s;
  for (s = 0; s < 8; s++) {
    var bay = document.querySelector('.parkBay[data-slot="' + s + '"]');
    var pivot = bay && bay.querySelector('.parkTurretBarrelPivot');
    var muz = bay && bay.querySelector('.parkTurretMuzzle');
    var cell = document.querySelector(
      '.tgCell[data-r="' + picks[s].tr + '"][data-c="' + picks[s].tc + '"]'
    );
    if (!pivot || !muz || !cell) {
      out.push({ slot: s, err: 'missing dom' });
      continue;
    }
    var wrap = bay.querySelector('.parkTurretWrap');
    if (wrap) wrap.style.animationPlayState = 'paused';
    var savedT = pivot.style.transition;
    var savedX = pivot.style.transform;
    pivot.style.transition = 'none';
    pivot.style.transform = 'rotate(0deg)';
    void pivot.offsetHeight;
    var pr = pivot.getBoundingClientRect();
    var px = (pr.left + pr.right) / 2;
    var py = pr.bottom;
    var mz = muz.getBoundingClientRect();
    var mx = (mz.left + mz.right) / 2;
    var my = (mz.top + mz.bottom) / 2;
    var theta0 = Math.atan2(my - py, mx - px);
    var cr = cell.getBoundingClientRect();
    var tx = (cr.left + cr.right) / 2;
    var ty = (cr.top + cr.bottom) / 2;
    var thetaT = Math.atan2(ty - py, tx - px);
    var expectAim = wrapDeg(((thetaT - theta0) * 180) / Math.PI);
    pivot.style.transition = savedT;
    pivot.style.transform = savedX;
    if (wrap) wrap.style.animationPlayState = '';
    void pivot.offsetHeight;
    var m = /rotate\(([-0-9.eE+]+)deg\)/.exec(pivot.style.transform || '');
    var actual = m ? parseFloat(m[1]) : 0;
    var diff = Math.abs(wrapDeg(actual - expectAim));
    if (diff > 10) out.push({ slot: s, expectAim: +expectAim.toFixed(2), actual: +actual.toFixed(2), diff: +diff.toFixed(2) });
  }
  return out;
}, sched.picks);

await browser.close();
server.close();

if (bad.length) {
  console.error('FAIL turret-parallel-aim', JSON.stringify(bad, null, 2));
  process.exit(1);
}

console.log('OK turret-parallel-aim (8 slots + renderTarget interleave)');
