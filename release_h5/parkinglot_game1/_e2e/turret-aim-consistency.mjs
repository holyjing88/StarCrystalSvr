/**
 * 自检：对每个带炮塔的车位槽位，用「归零测原点 + thetaT-theta0」旋转后，
 * 检查枪口方向与 pivot→目标 的方位差是否在容差内（多目标格多点抽样）。
 * 无炮塔则 SKIP。
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
await page.waitForSelector('.parkBay[data-slot="0"] .parkTurretBarrelPivot', { timeout: 15000 });
await page.waitForSelector('.targetGrid .tgCell:not(.empty)', { timeout: 15000 });

await page.waitForTimeout(400);

const result = await page.evaluate(() => {
  function wrapDeg(d) {
    d = d % 360;
    if (d > 180) d -= 360;
    if (d <= -180) d += 360;
    return d;
  }
  var pivots = document.querySelectorAll('.parkBay .parkTurretBarrelPivot');
  if (!pivots.length) return { skip: true };

  var grid = document.querySelector('.targetGrid');
  var cells = grid ? grid.querySelectorAll('.tgCell:not(.empty)') : [];
  if (!cells.length) return { skip: true, reason: 'no target cells' };

  var bad = [];
  var samples = 0;
  var slotList = [];

  for (var s = 0; s < 8; s++) {
    var bay = document.querySelector('.parkBay[data-slot="' + s + '"]');
    var pivot = bay && bay.querySelector('.parkTurretBarrelPivot');
    var muz = bay && bay.querySelector('.parkTurretMuzzle');
    if (!pivot || !muz) continue;

    slotList.push(s);
    var wrap = bay.querySelector('.parkTurretWrap');
    var savedPlay = wrap ? wrap.style.animationPlayState : '';
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

    var ni = Math.min(8, cells.length);
    var ki;
    for (ki = 0; ki < ni; ki++) {
      var cell = cells[ki];
      var cr = cell.getBoundingClientRect();
      var tx = (cr.left + cr.right) / 2;
      var ty = (cr.top + cr.bottom) / 2;
      var thetaT = Math.atan2(ty - py, tx - px);
      var aimDeg = wrapDeg(((thetaT - theta0) * 180) / Math.PI);

      pivot.style.transition = 'none';
      pivot.style.transform = 'rotate(' + aimDeg + 'deg)';
      void pivot.offsetHeight;
      var mz2 = muz.getBoundingClientRect();
      var mx2 = (mz2.left + mz2.right) / 2;
      var my2 = (mz2.top + mz2.bottom) / 2;
      var thetaGun = Math.atan2(my2 - py, mx2 - px);
      var errDeg = Math.abs(wrapDeg(((thetaGun - thetaT) * 180) / Math.PI));
      samples++;
      if (errDeg > 10) bad.push({ slot: s, cellIdx: ki, errDeg: +errDeg.toFixed(2) });
    }

    pivot.style.transition = savedT;
    pivot.style.transform = savedX;
    void pivot.offsetHeight;
    if (wrap) wrap.style.animationPlayState = savedPlay || '';
  }

  return { skip: false, bad, samples, slots: slotList };
});

await browser.close();
server.close();

if (result.skip) {
  console.log('SKIP turret-aim-consistency', result.reason || '(no pivot / no targets)');
  process.exit(0);
}

if (result.bad.length) {
  console.error('FAIL turret aim mismatch:', JSON.stringify(result.bad.slice(0, 12), null, 2));
  console.error('samples', result.samples, 'slots', result.slots);
  process.exit(1);
}

console.log(
  'OK turret-aim-consistency samples=' + result.samples + ' slots=[' + result.slots.join(',') + ']'
);
