/**
 * 浏览器验收：回归测试 → 客户端回归 → 点击运行，验证 UI 与 API 轮询。
 * 用法：先 npm run dev，再 node scripts/client-regression-acceptance.mjs
 */
import { chromium } from 'playwright';

const baseURL = (process.env.VITE_URL ?? 'http://localhost:5174').replace(/\/$/, '');
const headless = process.env.HEADLESS !== '0';
const maxWaitMs = Number(process.env.CLIENT_ACCEPTANCE_MAX_MS ?? 25 * 60 * 1000);

function log(msg) {
  console.log(`[client-acceptance] ${msg}`);
}

async function main() {
  const browser = await chromium.launch({ headless });
  const page = await browser.newPage();
  page.setDefaultTimeout(120000);

  const apiLog = [];
  page.on('response', async (res) => {
    const u = res.url();
    if (!u.includes('/dev/regression/client')) return;
    let body = '';
    try {
      body = await res.text();
    } catch {
      body = '';
    }
    apiLog.push({ status: res.status(), url: u.replace(baseURL, ''), body: body.slice(0, 500) });
  });

  log(`打开 ${baseURL}`);
  await page.goto(baseURL, { waitUntil: 'domcontentloaded' });

  log('切换到「回归测试」');
  await page.getByRole('button', { name: '回归测试' }).click();
  await page.getByRole('button', { name: '客户端回归' }).click();

  const runBtn = page.locator('.subpanel.active .card > button.primary');
  await runBtn.waitFor({ state: 'visible' });

  log('点击「运行客户端回归」');
  const clickAt = Date.now();
  await runBtn.click();

  await page.waitForFunction(
    () => {
      const btn = document.querySelector('.subpanel.active .card > button.primary');
      const t = btn?.textContent ?? '';
      return t.includes('运行中：') || t.startsWith('完成');
    },
    null,
    { timeout: 45000 },
  );
  const btnAfterClick = (await runBtn.textContent())?.trim() ?? '';
  log(`点击后 ${Date.now() - clickAt}ms 按钮: "${btnAfterClick}"`);

  if (!btnAfterClick.includes('运行中：') && !btnAfterClick.startsWith('完成')) {
    throw new Error(`按钮未进入运行中/完成状态: "${btnAfterClick}"`);
  }

  const post = apiLog.find((x) => x.url === '/dev/regression/client' || x.url.endsWith('/dev/regression/client'));
  if (!post) {
    log('API 日志: ' + JSON.stringify(apiLog, null, 2));
    throw new Error('未捕获 POST /dev/regression/client');
  }
  log(`POST 响应 HTTP ${post.status}: ${post.body.slice(0, 200)}`);
  if (post.status !== 202 && post.status !== 200) {
    throw new Error(`POST 非 200/202: ${post.status} ${post.body}`);
  }
  const postJson = JSON.parse(post.body || '{}');
  if (postJson.joined) log('已接续进行中的后台任务');

  log(`等待完成（最长 ${Math.round(maxWaitMs / 60000)} 分钟）…`);
  await page.waitForFunction(
    () => {
      const btn = document.querySelector('.subpanel.active .card > button.primary');
      return (btn?.textContent ?? '').startsWith('完成');
    },
    null,
    { timeout: maxWaitMs },
  );

  const summary = (await runBtn.textContent())?.trim() ?? '';
  log(`结束: ${summary}`);

  const rows = page.locator('.subpanel.active table.regression tbody tr');
  const count = await rows.count();
  let pass = 0;
  let fail = 0;
  for (let i = 0; i < Math.min(count, 5); i++) {
    const id = (await rows.nth(i).locator('td').nth(0).textContent())?.trim() ?? '';
    const result = (await rows.nth(i).locator('td').nth(4).textContent())?.trim() ?? '';
    const err = (await rows.nth(i).locator('td').nth(6).textContent())?.trim() ?? '';
    log(`  ${id} ${result} ${err.slice(0, 80)}`);
    if (result === 'PASS') pass++;
    else fail++;
  }
  if (count > 5) log(`  … 共 ${count} 行`);

  const logPre = page.locator('.subpanel.active pre.run-log');
  const logText = (await logPre.textContent())?.trim().slice(-500) ?? '';
  if (logText) log(`日志尾部: …${logText}`);

  await browser.close();

  const m = summary.match(/完成 (\d+)\/(\d+)/);
  const total = m ? Number(m[2]) : count;
  const passed = m ? Number(m[1]) : pass;
  if (passed === 0 && fail > 0) {
    process.exitCode = 1;
    throw new Error(`全部失败: ${summary}`);
  }
  log(`验收结束 ${passed}/${total} 通过（${Date.now() - clickAt}ms）`);
}

main().catch((e) => {
  console.error('[client-acceptance] 失败:', e.message);
  process.exit(1);
});
