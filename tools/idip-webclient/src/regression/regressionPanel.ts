import { checkPlayerApiReachable, IdipClient } from '../api/idipClient';
import type { CaseResult } from '../api/types';
import { REGRESSION_CASES } from '../tests/cases';
import { runRegression } from '../tests/runner';
import { uniqueGoTests } from './catalogUtils';
import { CLIENT_REGRESSION_CATALOG } from './clientCatalog';
import type { RegressionCatalogCase, RegressionRowResult } from './catalogTypes';
import {
  fetchClientRegressionJob,
  runClientRegressionRemote,
  runServerRegressionRemote,
} from './remoteRunner';
import type { ClientRegressionProgress } from './parseUnityUtpLog';
import { SERVER_REGRESSION_CATALOG } from './serverCatalog';

type ElFn = <K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
) => HTMLElementTagNameMap[K];

export function buildRegressionHub(
  el: ElFn,
  escapeHtml: (s: string) => string,
  getConfig: () => { baseUrl: string; idipKey: string },
  onClientChange: (c: IdipClient) => void,
): HTMLElement {
  const wrap = el('div', 'regression-hub');
  const subnav = el('nav', 'subtabs');
  const subPanels: Record<string, HTMLElement> = {};
  const subIds = ['idip', 'server', 'client'] as const;
  const subLabels: Record<(typeof subIds)[number], string> = {
    idip: 'IDIP 回归',
    server: '服务端回归',
    client: '客户端回归',
  };
  let activeSub: (typeof subIds)[number] = 'idip';

  const body = el('div', 'regression-body');

  for (const id of subIds) {
    const btn = el('button', id === activeSub ? 'active' : undefined, subLabels[id]);
    btn.type = 'button';
    btn.onclick = () => {
      activeSub = id;
      subnav.querySelectorAll('button').forEach((b, i) => {
        b.classList.toggle('active', subIds[i] === id);
      });
      Object.entries(subPanels).forEach(([k, p]) => {
        p.classList.toggle('active', k === id);
      });
    };
    subnav.append(btn);
    const panel = el('section', `subpanel${id === activeSub ? ' active' : ''}`);
    subPanels[id] = panel;
    body.append(panel);
  }

  subPanels.idip.append(
    el(
      'p',
      'hint',
      `浏览器内执行 ${REGRESSION_CASES.length} 条 IDIP 用例（对齐 idip-webclient · cases.ts）。` +
        `API Base 留空走 Vite 代理 /idip 与 /api。CLI：npm run regression`,
    ),
  );
  subPanels.idip.append(buildIdipRegressionPanel(el, escapeHtml, getConfig, onClientChange));

  subPanels.server.append(
    el(
      'p',
      'hint',
      `本机执行 server-go：go test ./internal/api/ ./internal/service/（${SERVER_REGRESSION_CATALOG.length} 条用例 · ${uniqueGoTests(SERVER_REGRESSION_CATALOG).length} 个 go test）。` +
        `需已配置 MySQL（AUTH_MYSQL_DSN 或 release/configs/starcrystal.json）。仅 dev 模式可用 /dev/regression/server。`,
    ),
  );
  subPanels.server.append(
    buildCatalogRegressionPanel(el, escapeHtml, SERVER_REGRESSION_CATALOG, '运行服务端回归', async () => {
      return runServerRegressionRemote(SERVER_REGRESSION_CATALOG);
    }),
  );

  subPanels.client.append(
    el(
      'p',
      'hint',
      `本机 Unity batch：EditMode + PlayMode（${CLIENT_REGRESSION_CATALOG.length} 条）。` +
        `运行前会自动结束占用 StarCrystal2022 的 Unity Editor；需 server-go。设 SC_FORCE_CLOSE_UNITY=0 可禁用。` +
        `CLI 见 StarCrystal2022/doc/客户端测试用例.md §1.3`,
    ),
  );
  const clientPanel = buildCatalogRegressionPanel(
    el,
    escapeHtml,
    CLIENT_REGRESSION_CATALOG,
    '运行客户端回归',
    async (onProgress) => runClientRegressionRemote(CLIENT_REGRESSION_CATALOG, onProgress),
    { liveProgress: true },
  );
  subPanels.client.append(clientPanel);
  void resumeClientRegressionPanelIfNeeded(clientPanel);

  wrap.append(subnav, body);
  return wrap;
}

function buildIdipRegressionPanel(
  el: ElFn,
  escapeHtml: (s: string) => string,
  getConfig: () => { baseUrl: string; idipKey: string },
  onClientChange: (c: IdipClient) => void,
): HTMLElement {
  const card = el('div', 'card');
  const runBtn = el('button', 'primary', '运行 IDIP 回归');
  const table = el('table', 'regression');
  const thead = el('thead');
  thead.innerHTML =
    '<tr><th>ID</th><th>接口</th><th>服务端用途</th><th>验证</th><th>结果</th><th>耗时</th><th>错误</th></tr>';
  const tbody = el('tbody');
  table.append(thead, tbody);
  const logPre = el('pre', 'response run-log hidden');
  card.append(runBtn, table, logPre);

  runBtn.onclick = async () => {
    runBtn.setAttribute('disabled', 'true');
    tbody.innerHTML = '';
    logPre.classList.add('hidden');
    const { baseUrl, idipKey } = getConfig();
    const local = new IdipClient(baseUrl, idipKey);
    onClientChange(local);
    const apiErr = await checkPlayerApiReachable(local);
    if (apiErr) {
      const tr = el('tr');
      tr.innerHTML = `<td colspan="7" class="fail">玩家 API 不可用：${escapeHtml(apiErr)}</td>`;
      tbody.append(tr);
      runBtn.removeAttribute('disabled');
      runBtn.textContent = '运行 IDIP 回归';
      return;
    }
    const results: CaseResult[] = await runRegression(local, REGRESSION_CASES, (id, i, n) => {
      runBtn.textContent = `运行中 ${i}/${n} (${id})…`;
    });
    fillIdipTable(el, escapeHtml, tbody, results);
    runBtn.removeAttribute('disabled');
    runBtn.textContent = `完成 ${results.filter((x) => x.passed).length}/${results.length} 通过`;
  };

  return card;
}

function fillIdipTable(
  el: ElFn,
  escapeHtml: (s: string) => string,
  tbody: HTMLElement,
  results: CaseResult[],
): void {
  for (let i = 0; i < results.length; i++) {
    const r = results[i]!;
    const c = REGRESSION_CASES[i]!;
    const tr = el('tr');
    tr.innerHTML = `<td>${r.id}</td><td>${escapeHtml(c.api)}</td><td>${escapeHtml(c.servicePurpose)}</td><td>${escapeHtml(c.verify)}</td><td class="${r.passed ? 'ok' : 'fail'}">${r.passed ? 'PASS' : 'FAIL'}</td><td>${r.durationMs}ms</td><td>${escapeHtml(r.error ?? '')}</td>`;
    tbody.append(tr);
  }
}

function fillCatalogRegressionProgressTable(
  el: ElFn,
  escapeHtml: (s: string) => string,
  catalog: RegressionCatalogCase[],
  tbody: HTMLElement,
  progress: ClientRegressionProgress,
): void {
  const byId = new Map(progress.cases.map((c) => [c.id, c]));
  tbody.innerHTML = '';
  for (const c of catalog) {
    const p = byId.get(c.id);
    const tr = el('tr');
    if (!p || p.status === 'pending') {
      tr.innerHTML = `<td>${c.id}</td><td>${escapeHtml(c.api)}</td><td>${escapeHtml(c.servicePurpose)}</td><td>${escapeHtml(c.verify)}</td><td>—</td><td>—</td><td>等待…</td>`;
    } else {
      const status = p.status === 'pass' ? 'PASS' : p.status === 'skip' ? 'SKIP' : 'FAIL';
      const statusClass = p.status === 'pass' ? 'ok' : p.status === 'skip' ? 'warn' : 'fail';
      tr.innerHTML = `<td>${c.id}</td><td>${escapeHtml(c.api)}</td><td>${escapeHtml(c.servicePurpose)}</td><td>${escapeHtml(c.verify)}</td><td class="${statusClass}">${status}</td><td>${p.durationMs != null ? `${p.durationMs}ms` : '—'}</td><td></td>`;
    }
    tbody.append(tr);
  }
}

function fillCatalogRegressionTable(
  el: ElFn,
  escapeHtml: (s: string) => string,
  catalog: RegressionCatalogCase[],
  tbody: HTMLElement,
  results: RegressionRowResult[],
  error?: string,
): void {
  const byId = new Map(results.map((r) => [r.id, r]));
  tbody.innerHTML = '';
  if (error && results.every((r) => !r.passed)) {
    const tr = el('tr');
    tr.innerHTML = `<td colspan="7" class="fail">${escapeHtml(error)}</td>`;
    tbody.append(tr);
  }
  for (const c of catalog) {
    const r = byId.get(c.id);
    const tr = el('tr');
    const passed = r?.passed === true;
    const skipped = r?.skipped === true;
    const status = skipped ? 'SKIP' : passed ? 'PASS' : 'FAIL';
    const statusClass = skipped ? 'warn' : passed ? 'ok' : 'fail';
    tr.innerHTML = `<td>${c.id}</td><td>${escapeHtml(c.api)}</td><td>${escapeHtml(c.servicePurpose)}</td><td>${escapeHtml(c.verify)}</td><td class="${statusClass}">${status}</td><td>${r?.durationMs != null ? `${r.durationMs}ms` : '—'}</td><td>${escapeHtml(r?.error ?? error ?? '')}</td>`;
    tbody.append(tr);
  }
}

async function resumeClientRegressionPanelIfNeeded(card: HTMLElement): Promise<void> {
  const runBtn = card.querySelector('button.primary');
  const tbody = card.querySelector('table.regression tbody');
  const logPre = card.querySelector('pre.run-log');
  if (!runBtn || !tbody || !logPre) return;

  let job: Awaited<ReturnType<typeof fetchClientRegressionJob>>;
  try {
    job = await fetchClientRegressionJob();
  } catch {
    return;
  }
  if (job.status === 'idle') return;

  const el = ((tag: 'tr', cls?: string) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    return n;
  }) as ElFn;
  const escapeHtml = (s: string) =>
    s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  if (job.status === 'done' && job.data) {
    fillCatalogRegressionTable(
      el,
      escapeHtml,
      CLIENT_REGRESSION_CATALOG,
      tbody as HTMLElement,
      job.data.results ?? [],
      job.data.error,
    );
    logPre.textContent = job.data.log ?? job.log ?? '';
    logPre.classList.remove('hidden');
    const passN = (job.data.results ?? []).filter((x) => x.passed).length;
    runBtn.textContent = `完成 ${passN}/${CLIENT_REGRESSION_CATALOG.length} 通过`;
    return;
  }

  if (job.status !== 'running') return;

  runBtn.setAttribute('disabled', 'true');
  tbody.innerHTML = '';
  logPre.classList.remove('hidden');
  logPre.textContent = job.log ?? '正在接续后台客户端回归…';
  if (job.progress) {
    fillCatalogRegressionProgressTable(el, escapeHtml, CLIENT_REGRESSION_CATALOG, tbody as HTMLElement, job.progress);
  }

  const { results, log, error } = await runClientRegressionRemote(
    CLIENT_REGRESSION_CATALOG,
    (label, logTail, progress) => {
      runBtn.textContent = `运行中：${label}`;
      if (logTail) logPre.textContent = logTail.slice(-8000);
      if (progress) {
        fillCatalogRegressionProgressTable(
          el,
          escapeHtml,
          CLIENT_REGRESSION_CATALOG,
          tbody as HTMLElement,
          progress,
        );
      }
    },
  );
  fillCatalogRegressionTable(
    el,
    escapeHtml,
    CLIENT_REGRESSION_CATALOG,
    tbody as HTMLElement,
    results,
    error,
  );
  if (log) {
    logPre.textContent = log;
    logPre.classList.remove('hidden');
  }
  const passN = results.filter((x) => x.passed).length;
  runBtn.removeAttribute('disabled');
  runBtn.textContent = `完成 ${passN}/${CLIENT_REGRESSION_CATALOG.length} 通过`;
}

function buildCatalogRegressionPanel(
  el: ElFn,
  escapeHtml: (s: string) => string,
  catalog: RegressionCatalogCase[],
  runLabel: string,
  runRemote: (
    onProgress?: (label: string, logTail?: string, progress?: ClientRegressionProgress) => void,
  ) => Promise<{ results: RegressionRowResult[]; log?: string; error?: string }>,
  options?: { liveProgress?: boolean },
): HTMLElement {
  const card = el('div', 'card');
  const runBtn = el('button', 'primary', runLabel);
  const table = el('table', 'regression');
  const thead = el('thead');
  thead.innerHTML =
    '<tr><th>ID</th><th>接口</th><th>用途</th><th>验证</th><th>结果</th><th>耗时</th><th>错误</th></tr>';
  const tbody = el('tbody');
  table.append(thead, tbody);
  const logPre = el('pre', 'response run-log hidden');
  card.append(runBtn, table, logPre);

  runBtn.onclick = async () => {
    runBtn.setAttribute('disabled', 'true');
    tbody.innerHTML = '';
    logPre.classList.add('hidden');
    runBtn.textContent = '已提交…';
    logPre.classList.remove('hidden');
    logPre.textContent =
      '正在启动客户端回归（EditMode + PlayMode；进度见浏览器控制台 F12 与 npm run dev 终端）…';
    if (options?.liveProgress) tbody.innerHTML = '';

    const { results, log, error } = await runRemote((label, logTail, progress) => {
      runBtn.textContent = `运行中：${label}`;
      if (logTail) {
        logPre.textContent = logTail.slice(-8000);
      }
      if (options?.liveProgress && progress) {
        fillCatalogRegressionProgressTable(el, escapeHtml, catalog, tbody, progress);
      }
    });
    fillCatalogRegressionTable(el, escapeHtml, catalog, tbody, results, error);

    if (log) {
      logPre.textContent = log;
      logPre.classList.remove('hidden');
    }
    const passN = results.filter((x) => x.passed).length;
    runBtn.removeAttribute('disabled');
    runBtn.textContent = `完成 ${passN}/${catalog.length} 通过`;
  };

  return card;
}
