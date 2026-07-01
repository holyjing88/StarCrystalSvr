/**
 * 浏览器验收：打开 IDIP 运营台 → 连通性检测 → 回归测试 11 条。
 * 用法：先 npm run dev，再 node scripts/browser-acceptance.mjs
 */
import { chromium } from 'playwright';

const baseURL = (process.env.VITE_URL ?? 'http://localhost:5175').replace(/\/$/, '');
const idipKey = process.env.IDIP_KEY ?? 'change-me-in-production';
const headless = process.env.HEADLESS !== '0';

async function main() {
  const browser = await chromium.launch({ headless });
  const page = await browser.newPage();
  page.setDefaultTimeout(180000);
  const log = (msg) => console.log(`[acceptance] ${msg}`);

  log(`打开 ${baseURL}`);
  await page.goto(baseURL, { waitUntil: 'networkidle' });

  const baseInput = page.locator('label:has-text("API Base") input');
  const keyInput = page.locator('label:has-text("X-IDIP-Key") input');
  await baseInput.fill('');
  await keyInput.fill(idipKey);

  log('点击「连通性检测」');
  await page.getByRole('button', { name: '连通性检测' }).click();
  const ping = page.locator('header .status-pill');
  await page.waitForFunction(
    () => {
      const t = document.querySelector('header .status-pill')?.textContent?.trim() ?? '';
      if (t === '检测中…' || t === '—' || t === '') return false;
      return true;
    },
    { timeout: 60000 },
  );
  const pingText = (await ping.textContent())?.trim() ?? '';
  log(`连通性: ${pingText}`);
  if (!pingText.includes('IDIP+API OK')) {
    throw new Error(`连通性检测未通过: "${pingText}"`);
  }

  log('切换到「回归测试」');
  await page.getByRole('button', { name: '回归测试' }).click();

  log('确认 IDIP 回归子 Tab');
  await page.getByRole('button', { name: 'IDIP 回归' }).click();

  log('点击「运行 IDIP 回归」');
  const runBtn = page.locator('section.panel.active .subpanel.active .card > button.primary').first();
  await runBtn.click();

  await page.waitForFunction(
    () => {
      const btn = document.querySelector('section.panel.active .card > button.primary');
      return (btn?.textContent ?? '').startsWith('完成');
    },
    { timeout: 180000 },
  );

  const summary = (await runBtn.textContent())?.trim() ?? '';
  log(`回归结束: ${summary}`);

  const rows = page.locator('table.regression tbody tr');
  const count = await rows.count();
  let pass = 0;
  let fail = 0;
  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const id = (await row.locator('td').nth(0).textContent())?.trim() ?? '';
    const result = (await row.locator('td').nth(4).textContent())?.trim() ?? '';
    const err = (await row.locator('td').nth(6).textContent())?.trim() ?? '';
    if (result === 'PASS') pass++;
    else {
      fail++;
      log(`  FAIL ${id}: ${err}`);
    }
  }

  log(`表格: ${pass} PASS, ${fail} FAIL, 共 ${count} 行`);
  await browser.close();

  const m = summary.match(/完成 (\d+)\/(\d+)/);
  if (!m || m[1] !== m[2]) {
    process.exitCode = 1;
    throw new Error(`未全部通过: ${summary}`);
  }
  if (fail > 0 || pass !== Number(m[2])) {
    process.exitCode = 1;
    throw new Error(`表格与汇总不一致: ${summary}, ${pass} PASS / ${fail} FAIL`);
  }
  log('验收通过');
}

main().catch((e) => {
  console.error('[acceptance] 失败:', e.message);
  process.exit(1);
});
