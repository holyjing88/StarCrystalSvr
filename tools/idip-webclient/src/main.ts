import './style.css';
import { checkPlayerApiReachable, IdipClient } from './api/idipClient';
import type { GoldSetUserRequest } from './api/types';
import {
  clearSession,
  getSessionToken,
  getSessionUsername,
  setHeartbeatIntervalSec,
  setOnSessionExpired,
  startHeartbeat,
  stopHeartbeat,
} from './auth/session';
import { ensureServerReachable } from './tests/runner';
import { buildRegressionHub } from './regression/regressionPanel';
import { buildGamesPanel } from './games/gamesPanel';

const app = document.getElementById('app')!;
let client = IdipClient.fromStorage();
let lastResponse = '';

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

function showResponse(label: string, httpStatus: number, body: unknown): void {
  lastResponse = JSON.stringify({ label, httpStatus, body }, null, 2);
  document.querySelectorAll('pre.response').forEach((p) => {
    p.textContent = lastResponse;
  });
}

function wireClientSessionHandlers(): void {
  client.setSessionExpiredHandler(() => {
    stopHeartbeat();
    boot();
  });
  setOnSessionExpired(() => boot());
}

function renderLogin(): void {
  app.innerHTML = '';
  const card = el('div', 'card login-card');
  card.append(el('h1', undefined, 'StarCrystal 运营登录'));

  const userLabel = el('label');
  userLabel.append('账号', el('input'));
  const userInput = userLabel.querySelector('input')!;
  userInput.autocomplete = 'username';

  const passLabel = el('label');
  passLabel.append('密码', el('input'));
  const passInput = passLabel.querySelector('input')!;
  passInput.type = 'password';
  passInput.autocomplete = 'current-password';

  const err = el('p', 'login-error');
  const submit = el('button', 'primary login-submit', '登录');
  const doLogin = async () => {
    err.textContent = '';
    client.saveToStorage();
    const { ok, httpStatus, envelope } = await client.login(
      userInput.value.trim(),
      passInput.value,
    );
    if (!ok) {
      err.textContent =
        envelope.code === 1429
          ? '登录过于频繁，请稍后再试'
          : `登录失败 (${httpStatus} code=${envelope.code}) ${envelope.message}`;
      return;
    }
    if (envelope.data?.heartbeatIntervalSec) {
      setHeartbeatIntervalSec(envelope.data.heartbeatIntervalSec);
    }
    wireClientSessionHandlers();
    boot();
  };
  submit.onclick = () => void doLogin();
  passInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') void doLogin();
  });
  card.append(userLabel, passLabel, err, submit);
  app.append(card);
  userInput.focus();
}

function render(): void {
  app.innerHTML = '';

  const header = el('header');
  const title = el('h1', undefined, 'StarCrystal IDIP 运营台');
  const configRow = el('div', 'config-row');

  const baseLabel = el('label');
  baseLabel.append('API Base', el('input', 'wide'));
  const baseInput = baseLabel.querySelector('input')!;
  baseInput.placeholder = '留空=代理 /idip 与 /api → 8080';
  baseInput.value = client.baseUrl;

  const userSpan = el('span', 'status-pill ok', getSessionUsername() || '—');
  const logoutBtn = el('button', undefined, '退出');
  logoutBtn.onclick = async () => {
    stopHeartbeat();
    await client.logout();
    clearSession();
    boot();
  };

  const pingBtn = el('button', 'primary', '连通性检测');
  const pingStatus = el('span', 'status-pill', '—');

  pingBtn.onclick = async () => {
    client.baseUrl = baseInput.value.trim();
    client.saveToStorage();
    pingStatus.textContent = '检测中…';
    pingStatus.className = 'status-pill';
    const idipErr = await ensureServerReachable(client);
    if (idipErr) {
      pingStatus.textContent = idipErr.slice(0, 48);
      pingStatus.className = 'status-pill fail';
      return;
    }
    const apiErr = await checkPlayerApiReachable(client);
    if (apiErr) {
      pingStatus.textContent = 'IDIP OK；/api 失败';
      pingStatus.className = 'status-pill fail';
      return;
    }
    pingStatus.textContent = 'IDIP+API OK';
    pingStatus.className = 'status-pill ok';
  };

  configRow.append(baseLabel, userSpan, logoutBtn, pingBtn, pingStatus);
  header.append(title, configRow);

  const nav = el('nav', 'tabs');
  const panels: Record<string, HTMLElement> = {};
  const tabIds = ['gold', 'welfare', 'tasks', 'games', 'audit', 'regression'] as const;
  const tabLabels: Record<(typeof tabIds)[number], string> = {
    gold: '金币',
    welfare: '福利结算',
    tasks: '福利任务',
    games: '游戏列表',
    audit: '操作日志',
    regression: '回归测试',
  };

  let activeTab: (typeof tabIds)[number] = 'gold';

  const main = el('main');

  for (const id of tabIds) {
    const btn = el('button', id === activeTab ? 'active' : undefined, tabLabels[id]);
    btn.onclick = () => {
      activeTab = id;
      nav.querySelectorAll('button').forEach((b, i) => {
        b.classList.toggle('active', tabIds[i] === id);
      });
      Object.entries(panels).forEach(([k, p]) => p.classList.toggle('active', k === id));
    };
    nav.append(btn);
    panels[id] = el('section', `panel${id === activeTab ? ' active' : ''}`);
    main.append(panels[id]);
  }

  // —— Gold ——
  panels.gold.append(
    el('p', 'hint', 'POST /idip/v1/gold/set-user · GET month-user'),
  );
  panels.gold.append(buildGoldPanel());

  // —— Welfare ——
  panels.welfare.append(
    el('p', 'hint', '月 Token 池 · 月末结算（需 Redis/MySQL 经济模块）'),
  );
  panels.welfare.append(buildWelfarePanel());

  // —— Tasks ——
  panels.tasks.append(el('p', 'hint', '任务目录 · 分层开关 · 单任务覆盖 · 用户进度'));
  panels.tasks.append(buildTasksPanel());

  panels.games.append(el('p', 'hint', 'games.json 维护 · offline 可删除 · H5 上传'));
  panels.games.append(buildGamesPanel(el, client, showResponse, { field, wrapLabel }));

  panels.audit.append(el('p', 'hint', 'GET /idip/v1/audit/logs（只读）'));
  panels.audit.append(buildAuditPanel());

  // —— Regression（IDIP / 服务端 go test / 客户端 Unity）——
  panels.regression.append(
    buildRegressionHub(
      el,
      escapeHtml,
      () => ({
        baseUrl: baseInput.value.trim(),
        idipKey: client.idipKey,
      }),
      (c) => {
        client = c;
      },
    ),
  );

  app.append(header, nav, main);
}

function buildGoldPanel(): HTMLElement {
  const card = el('div', 'card');
  card.append(el('h2', undefined, '改用户金币 (IDP-003)'));
  const form = el('form', 'api-form') as HTMLFormElement;
  const accountId = field('accountId', 'accountId', 'acc_test');
  const op = el('select');
  ['add', 'deduct', 'set'].forEach((v) => op.append(new Option(v, v)));
  const amount = field('amount', 'amount', '100');
  const biz = field('bizType', 'bizType（可选）', 'idip_manual');
  const submit = el('button', 'primary', 'POST set-user');
  form.append(wrapLabel('op', op), accountId, amount, biz, submit);
  submit.onclick = async (e) => {
    e.preventDefault();
    const { httpStatus, envelope } = await client.goldSetUser({
      accountId: (accountId.querySelector('input') as HTMLInputElement).value,
      op: (op as HTMLSelectElement).value as GoldSetUserRequest['op'],
      amount: Number((amount.querySelector('input') as HTMLInputElement).value),
      bizType: (biz.querySelector('input') as HTMLInputElement).value || undefined,
    });
    showResponse('gold/set-user', httpStatus, envelope);
  };
  card.append(form);

  const monthCard = el('div', 'card');
  monthCard.append(el('h2', undefined, '月金币 delta (IDP-004)'));
  const monthAcc = field('monthAccountId', 'accountId', 'acc_test');
  const monthBtn = el('button', 'primary', 'GET month-user');
  monthBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.goldMonthUser(
      (monthAcc.querySelector('input') as HTMLInputElement).value,
    );
    showResponse('gold/month-user', httpStatus, envelope);
  };
  monthCard.append(monthAcc, monthBtn);

  const recalcCard = el('div', 'card');
  recalcCard.append(el('h2', undefined, '重算 (IDP-010)'));
  const recalcBtn = el('button', 'primary', 'POST recalc-server-delta-total');
  recalcBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.goldRecalcServerDelta();
    showResponse('recalc-server-delta-total', httpStatus, envelope);
  };
  recalcCard.append(recalcBtn);

  const wrap = el('div');
  const pre = el('pre', 'response', lastResponse || '{}');
  wrap.append(card, monthCard, recalcCard, pre);
  return wrap;
}

function buildWelfarePanel(): HTMLElement {
  const wrap = el('div');
  const poolCard = el('div', 'card');
  poolCard.append(el('h2', undefined, '月 Token 池 (IDP-005)'));
  const getBtn = el('button', 'primary', 'GET month-token-pool');
  const poolInput = field('pool', 'pool', '100000');
  const setBtn = el('button', 'primary', 'POST set-month-token-pool');
  poolCard.append(getBtn, poolInput, setBtn);

  const settleCard = el('div', 'card');
  settleCard.append(el('h2', undefined, '月末结算 (IDP-006)'));
  const force = el('input');
  force.type = 'checkbox';
  const settleBtn = el('button', 'primary', 'POST run-monthly-settlement');
  settleCard.append(wrapLabel('force', force), settleBtn);

  const pre = el('pre', 'response', lastResponse || '{}');

  getBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.getMonthTokenPool();
    showResponse('month-token-pool GET', httpStatus, envelope);
  };
  setBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.setMonthTokenPool(
      Number((poolInput.querySelector('input') as HTMLInputElement).value),
    );
    showResponse('set-month-token-pool', httpStatus, envelope);
  };
  settleBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.runMonthlySettlement(
      (force as HTMLInputElement).checked,
    );
    showResponse('run-monthly-settlement', httpStatus, envelope);
  };

  wrap.append(poolCard, settleCard, pre);
  return wrap;
}

function buildAuditPanel(): HTMLElement {
  const wrap = el('div');
  const card = el('div', 'card');
  card.append(el('h2', undefined, '操作日志'));
  const action = field('auditAction', 'action 筛选', '');
  const gameId = field('auditGameId', 'gameId 筛选', '');
  const listPre = el('pre', 'response', '[]');
  const refreshBtn = el('button', 'primary', '刷新日志');
  refreshBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.auditLogs({
      limit: 50,
      offset: 0,
      action: (action.querySelector('input') as HTMLInputElement).value || undefined,
      gameId: (gameId.querySelector('input') as HTMLInputElement).value || undefined,
    });
    showResponse('audit/logs', httpStatus, envelope);
    listPre.textContent = JSON.stringify(envelope.data?.list ?? [], null, 2);
  };
  card.append(action, gameId, refreshBtn, listPre);
  wrap.append(card);
  return wrap;
}

function buildTasksPanel(): HTMLElement {
  const wrap = el('div');
  const defsCard = el('div', 'card');
  defsCard.append(el('h2', undefined, '任务目录 (TSK-A-001)'));
  const defsBtn = el('button', 'primary', 'GET definitions');
  defsCard.append(defsBtn);

  const tierCard = el('div', 'card');
  tierCard.append(el('h2', undefined, '分层开关 (TSK-S-006)'));
  const p0 = checkboxField('p0Enabled', true);
  const p1 = checkboxField('p1Enabled', false);
  const p2 = checkboxField('p2Enabled', false);
  const tierBtn = el('button', 'primary', 'POST tier-policy');
  tierCard.append(p0, p1, p2, tierBtn);

  const upsertCard = el('div', 'card');
  upsertCard.append(el('h2', undefined, 'definition/upsert (TSK-A-002)'));
  const taskId = field('taskId', 'taskId', 'daily_free_claim');
  const reward = field('rewardGold', 'rewardGold', '51');
  const upsertBtn = el('button', 'primary', 'POST upsert');
  upsertCard.append(taskId, reward, upsertBtn);

  const progCard = el('div', 'card');
  progCard.append(el('h2', undefined, 'user-progress (运营)'));
  const acc = field('accountId', 'accountId', 'guest_smoke');
  const progBtn = el('button', 'primary', 'GET user-progress');
  progCard.append(acc, progBtn);

  const pre = el('pre', 'response', lastResponse || '{}');

  defsBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.taskDefinitions();
    showResponse('definitions', httpStatus, envelope);
  };
  tierBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.taskTierPolicy({
      p0Enabled: (p0.querySelector('input') as HTMLInputElement).checked,
      p1Enabled: (p1.querySelector('input') as HTMLInputElement).checked,
      p2Enabled: (p2.querySelector('input') as HTMLInputElement).checked,
    });
    showResponse('tier-policy', httpStatus, envelope);
  };
  upsertBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.taskDefinitionUpsert({
      taskId: (taskId.querySelector('input') as HTMLInputElement).value,
      rewardGold: Number((reward.querySelector('input') as HTMLInputElement).value),
    });
    showResponse('upsert', httpStatus, envelope);
  };
  progBtn.onclick = async () => {
    const { httpStatus, envelope } = await client.taskUserProgress(
      (acc.querySelector('input') as HTMLInputElement).value,
    );
    showResponse('user-progress', httpStatus, envelope);
  };

  wrap.append(defsCard, tierCard, upsertCard, progCard, pre);
  return wrap;
}

function field(name: string, labelText: string, value: string): HTMLElement {
  const label = el('label');
  label.append(labelText, el('input'));
  const input = label.querySelector('input')!;
  input.name = name;
  input.value = value;
  return label;
}

function checkboxField(name: string, checked: boolean): HTMLElement {
  const label = el('label');
  const input = el('input');
  input.type = 'checkbox';
  input.name = name;
  (input as HTMLInputElement).checked = checked;
  label.append(name, input);
  return label;
}

function wrapLabel(text: string, control: HTMLElement): HTMLElement {
  const label = el('label');
  label.append(text, control);
  return label;
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function boot(): void {
  client = IdipClient.fromStorage();
  if (!getSessionToken()) {
    stopHeartbeat();
    renderLogin();
    return;
  }
  wireClientSessionHandlers();
  startHeartbeat(() => client.heartbeat());
  render();
}

boot();
