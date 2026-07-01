import type { IdipClient } from '../api/idipClient';
import type { IdipGameRow } from '../api/types';
import {
  parseMinigameVersionFromPackageFileName,
  setH5MetaFieldValue,
  syncH5GameIdDatalist,
  wireGameIdMetaReload,
} from './h5UploadUtils';

type ShowResponse = (label: string, httpStatus: number, body: unknown) => void;

type ElFn = <K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
) => HTMLElementTagNameMap[K];

const GAME_COLUMNS: {
  key: string;
  label: string;
  readonly?: boolean;
  type?: 'text' | 'number' | 'status';
}[] = [
  { key: 'gameId', label: 'gameId', readonly: true },
  { key: 'entryUrl', label: 'entryUrl', readonly: true },
  { key: 'name', label: '名称' },
  { key: 'nameEn', label: 'nameEn' },
  { key: 'nameUr', label: 'nameUr' },
  { key: 'status', label: '状态', type: 'status' },
  { key: 'sort', label: '排序', type: 'number' },
  { key: 'iconLink', label: 'iconLink' },
  { key: 'coverUrl', label: 'coverUrl' },
  { key: 'minAppVersion', label: 'minAppVersion' },
  { key: 'channels', label: 'channels' },
  { key: 'downloadUrl', label: 'downloadUrl', readonly: true },
  { key: 'packageBytes', label: 'packageBytes', readonly: true, type: 'number' },
  { key: 'downloadSha256', label: 'downloadSha256', readonly: true },
];

const H5_META_FIELDS: {
  key: string;
  label: string;
  type?: 'text' | 'number' | 'status' | 'entryType';
  required?: boolean;
  defaultValue?: string | number;
}[] = [
  { key: 'gameId', label: 'gameId', required: true, defaultValue: 'g001' },
  {
    key: 'minigameVersion',
    label: 'contentVersion',
    required: true,
    defaultValue: '1.0.0.1',
  },
  { key: 'name', label: '名称', required: true, defaultValue: '测试游戏' },
  { key: 'nameEn', label: 'nameEn' },
  { key: 'nameUr', label: 'nameUr' },
  { key: 'note', label: 'note' },
  { key: 'noteEn', label: 'noteEn' },
  { key: 'noteUr', label: 'noteUr' },
  { key: 'entryType', label: 'entryType', type: 'entryType', defaultValue: 'h5' },
  { key: 'status', label: '状态', type: 'status', defaultValue: 'online' },
  { key: 'sort', label: '排序', type: 'number', defaultValue: 1 },
  { key: 'channels', label: 'channels' },
  { key: 'iconLink', label: 'iconLink' },
  { key: 'coverUrl', label: 'coverUrl' },
  { key: 'minAppVersion', label: 'minAppVersion' },
];

function makeCellInput(
  el: ElFn,
  col: (typeof GAME_COLUMNS)[number],
  value: string,
): HTMLElement {
  if (col.readonly) {
    const span = el('span', 'games-cell-readonly', value);
    span.dataset.field = col.key;
    return span;
  }
  if (col.type === 'status') {
    const sel = el('select');
    sel.dataset.field = col.key;
    for (const s of ['online', 'offline']) {
      const opt = new Option(s, s);
      if (s === value) opt.selected = true;
      sel.append(opt);
    }
    return sel;
  }
  const inp = el('input');
  inp.dataset.field = col.key;
  inp.type = col.type === 'number' ? 'number' : 'text';
  inp.value = value;
  if (col.type === 'number') inp.className = 'games-num-input';
  return inp;
}

function rowFieldValue(row: IdipGameRow, key: string): string {
  const v = (row as unknown as Record<string, unknown>)[key];
  if (v == null) return '';
  if (Array.isArray(v)) return v.join(', ');
  return String(v);
}

function makeMetaControl(
  el: ElFn,
  field: (typeof H5_META_FIELDS)[number],
): { row: HTMLElement; input: HTMLInputElement | HTMLSelectElement } {
  const row = el('div', 'meta-field-row');
  const label = el('label', 'meta-field-label', field.label);
  let input: HTMLInputElement | HTMLSelectElement;
  if (field.type === 'status') {
    input = el('select', 'meta-field-control');
    for (const s of ['online', 'offline']) {
      input.append(new Option(s, s));
    }
    input.value = String(field.defaultValue ?? 'online');
  } else if (field.type === 'entryType') {
    input = el('select', 'meta-field-control');
    input.append(new Option('h5', 'h5'));
    input.value = 'h5';
  } else {
    input = el('input', 'meta-field-control');
    input.type = field.type === 'number' ? 'number' : 'text';
    input.value =
      field.defaultValue != null ? String(field.defaultValue) : '';
  }
  input.dataset.metaKey = field.key;
  row.append(label, input);
  return { row, input };
}

function collectBatchItemFromRow(tr: HTMLElement): Record<string, unknown> | null {
  const item: Record<string, unknown> = {};
  tr.querySelectorAll('[data-field]').forEach((node) => {
    const key = node.getAttribute('data-field');
    if (!key) return;
    if (node instanceof HTMLInputElement) {
      const raw = node.value.trim();
      if (key === 'sort') {
        const n = raw === '' ? 0 : Number(raw);
        item.sort = Number.isNaN(n) ? 0 : n;
        return;
      }
      // 空字符串也要提交，服务端 batch-upsert 用于清空 coverUrl 等可选字段
      item[key] = raw;
      return;
    }
    if (node instanceof HTMLSelectElement) {
      item[key] = node.value.trim();
      return;
    }
    if (node instanceof HTMLSpanElement && key === 'gameId') {
      item.gameId = node.textContent?.trim() ?? '';
    }
  });
  if (typeof item.gameId === 'string' && item.gameId) return item;
  return null;
}

function collectBatchItems(tbody: HTMLElement): Array<Record<string, unknown>> {
  const items: Array<Record<string, unknown>> = [];
  for (const tr of tbody.querySelectorAll('tr')) {
    const item = collectBatchItemFromRow(tr);
    if (item) items.push(item);
  }
  return items;
}

function collectH5Meta(form: HTMLElement): Record<string, unknown> | null {
  const meta: Record<string, unknown> = {};
  for (const field of H5_META_FIELDS) {
    const ctrl = form.querySelector(`[data-meta-key="${field.key}"]`) as
      | HTMLInputElement
      | HTMLSelectElement
      | null;
    if (!ctrl) continue;
    const raw = ctrl.value.trim();
    if (field.required && !raw) return null;
    if (!raw && !field.required) continue;
    if (field.type === 'number') {
      const n = Number(raw);
      if (!Number.isNaN(n)) meta[field.key] = n;
    } else {
      meta[field.key] = raw;
    }
  }
  if (!meta.entryType) meta.entryType = 'h5';
  return meta;
}

function renderListTable(
  el: ElFn,
  tbody: HTMLElement,
  rows: IdipGameRow[],
): void {
  tbody.replaceChildren();
  if (rows.length === 0) {
    const tr = el('tr');
    const td = el('td');
    td.colSpan = GAME_COLUMNS.length;
    td.className = 'games-empty';
    td.textContent = '（暂无数据，点击刷新列表）';
    tr.append(td);
    tbody.append(tr);
    return;
  }
  for (const row of rows) {
    const tr = el('tr');
    for (const col of GAME_COLUMNS) {
      const td = el('td');
      const text = rowFieldValue(row, col.key);
      if (col.readonly) {
        td.className = 'games-cell-readonly';
      }
      td.textContent = text;
      tr.append(td);
    }
    tbody.append(tr);
  }
}

function renderBatchTable(
  el: ElFn,
  tbody: HTMLElement,
  rows: IdipGameRow[],
  onSubmitRow: (tr: HTMLTableRowElement) => void,
): void {
  tbody.replaceChildren();
  const colSpan = GAME_COLUMNS.length + 1;
  if (rows.length === 0) {
    const tr = el('tr');
    const td = el('td');
    td.colSpan = colSpan;
    td.className = 'games-empty';
    td.textContent = '（请先刷新列表）';
    tr.append(td);
    tbody.append(tr);
    return;
  }
  for (const row of rows) {
    const tr = el('tr') as HTMLTableRowElement;
    for (const col of GAME_COLUMNS) {
      const td = el('td');
      td.append(makeCellInput(el, col, rowFieldValue(row, col.key)));
      tr.append(td);
    }
    const actionTd = el('td', 'games-action-cell');
    const submitBtn = el('button', 'games-row-submit', '提交');
    const rowStatus = el('span', 'games-row-status');
    submitBtn.onclick = () => onSubmitRow(tr);
    actionTd.append(submitBtn, rowStatus);
    tr.append(actionTd);
    tbody.append(tr);
  }
}

function buildTable(
  el: ElFn,
  columns: { label: string }[],
  tbodyClass: string,
): { table: HTMLTableElement; tbody: HTMLElement } {
  const table = el('table', 'games-table');
  const thead = el('thead');
  const headRow = el('tr');
  for (const col of columns) {
    const th = el('th', undefined, col.label);
    headRow.append(th);
  }
  thead.append(headRow);
  const tbody = el('tbody', tbodyClass);
  table.append(thead, tbody);
  return { table, tbody };
}

export function buildGamesPanel(
  el: ElFn,
  client: IdipClient,
  showResponse: ShowResponse,
  deps: {
    field: (name: string, labelText: string, value: string) => HTMLElement;
    wrapLabel: (text: string, control: HTMLElement) => HTMLElement;
  },
): HTMLElement {
  const wrap = el('div');
  let cachedVersion = '';
  let cachedRows: IdipGameRow[] = [];

  const versionSpan = el('span', 'games-version', 'configVersion: —');

  // —— 游戏列表（只读）——
  const listCard = el('div', 'card');
  listCard.append(el('h2', undefined, '游戏列表'));
  const { table: listTable, tbody: listTbody } = buildTable(el, GAME_COLUMNS, 'games-list-body');
  const listScroll = el('div', 'games-table-scroll');
  listScroll.append(listTable);
  const refreshBtn = el('button', 'primary', '刷新列表');
  listCard.append(refreshBtn, versionSpan, listScroll);

  let uploadFormSync: ((rows: IdipGameRow[]) => void) | null = null;

  const syncTables = (rows: IdipGameRow[]) => {
    cachedRows = rows;
    renderListTable(el, listTbody, rows);
    renderBatchTable(el, batchTbody, rows, submitSingleRow);
    uploadFormSync?.(rows);
  };

  const doRefresh = async () => {
    const { httpStatus, envelope } = await client.gamesList();
    showResponse('games/list', httpStatus, envelope);
    cachedVersion = envelope.data?.configVersion ?? '';
    versionSpan.textContent = cachedVersion
      ? `configVersion: ${cachedVersion.slice(0, 16)}…`
      : 'configVersion: —';
    syncTables(envelope.data?.list ?? []);
  };
  refreshBtn.onclick = () => void doRefresh();

  // —— 批量更新（可编辑列表项）——
  const batchCard = el('div', 'card');
  batchCard.append(el('h2', undefined, '批量更新 (batch-upsert)'));
  batchCard.append(
    el('p', 'hint', '每行对应一条游戏；可整表提交，或点击行末「提交」单项更新。'),
  );
  const batchColumns = [...GAME_COLUMNS, { label: '操作' }];
  const { table: batchTable, tbody: batchTbody } = buildTable(el, batchColumns, 'games-batch-body');
  const batchScroll = el('div', 'games-table-scroll');
  batchScroll.append(batchTable);
  const batchFeedback = el('p', 'batch-feedback hidden');
  const batchBtn = el('button', 'primary', '全部提交');
  batchCard.append(batchScroll, batchBtn, batchFeedback);

  const showBatchFeedback = (ok: boolean, message: string) => {
    batchFeedback.className = ok ? 'batch-feedback ok' : 'batch-feedback fail';
    batchFeedback.textContent = message;
  };

  const applyUpsertResult = (
    ok: boolean,
    envelope: { data?: { configVersion?: string } },
  ) => {
    if (ok && envelope.data?.configVersion) {
      cachedVersion = envelope.data.configVersion;
      versionSpan.textContent = `configVersion: ${cachedVersion.slice(0, 16)}…`;
    }
    return ok;
  };

  const runBatchUpsert = async (
    items: Array<Record<string, unknown>>,
    label: string,
  ): Promise<boolean> => {
    if (!cachedVersion) {
      showBatchFeedback(false, '请先刷新列表');
      return false;
    }
    if (items.length === 0) {
      showBatchFeedback(false, '没有可提交的行');
      return false;
    }
    const { httpStatus, envelope } = await client.gamesBatchUpsert({
      expectedConfigVersion: cachedVersion,
      items: items as Array<{ gameId: string }>,
    });
    showResponse('games/batch-upsert', httpStatus, envelope);
    const ok = httpStatus === 200 && envelope.code === 0;
    if (ok) {
      showBatchFeedback(true, `${label}成功（${items.length} 条）`);
      applyUpsertResult(ok, envelope);
      await doRefresh();
    } else {
      const detail = envelope.message || `HTTP ${httpStatus}`;
      showBatchFeedback(false, `${label}失败：${detail}`);
    }
    return ok;
  };

  async function submitSingleRow(tr: HTMLTableRowElement) {
    const rowStatus = tr.querySelector('.games-row-status') as HTMLElement | null;
    const submitBtn = tr.querySelector('.games-row-submit') as HTMLButtonElement | null;
    const item = collectBatchItemFromRow(tr);
    if (!item) {
      if (rowStatus) {
        rowStatus.className = 'games-row-status fail';
        rowStatus.textContent = '无效';
      }
      showBatchFeedback(false, '该行 gameId 无效');
      return;
    }
    const gameId = String(item.gameId);
    if (submitBtn) submitBtn.disabled = true;
    if (rowStatus) {
      rowStatus.className = 'games-row-status';
      rowStatus.textContent = '提交中…';
    }
    if (!cachedVersion) {
      showBatchFeedback(false, '请先刷新列表');
      if (submitBtn) submitBtn.disabled = false;
      if (rowStatus) rowStatus.textContent = '';
      return;
    }
    const { httpStatus, envelope } = await client.gamesBatchUpsert({
      expectedConfigVersion: cachedVersion,
      items: [item as { gameId: string }],
    });
    showResponse(`games/batch-upsert:${gameId}`, httpStatus, envelope);
    const ok = httpStatus === 200 && envelope.code === 0;
    if (rowStatus) {
      rowStatus.className = ok ? 'games-row-status ok' : 'games-row-status fail';
      rowStatus.textContent = ok ? '成功' : '失败';
    }
    if (ok) {
      showBatchFeedback(true, `${gameId} 更新成功`);
      applyUpsertResult(ok, envelope);
      await doRefresh();
    } else {
      const detail = envelope.message || `HTTP ${httpStatus}`;
      showBatchFeedback(false, `${gameId} 更新失败：${detail}`);
    }
    if (submitBtn) submitBtn.disabled = false;
  }

  batchBtn.onclick = async () => {
    batchBtn.disabled = true;
    try {
      await runBatchUpsert(collectBatchItems(batchTbody), '批量更新');
    } finally {
      batchBtn.disabled = false;
    }
  };

  // —— 删除 ——
  const delCard = el('div', 'card');
  delCard.append(el('h2', undefined, '删除 offline 游戏'));
  const gid = deps.field('delGameId', 'gameId', 'g001');
  const delH5 = el('input');
  delH5.type = 'checkbox';
  const delBtn = el('button', 'primary', 'POST delete');
  delCard.append(gid, deps.wrapLabel('deleteH5Dir', delH5), delBtn);
  delBtn.onclick = async () => {
    if (!cachedVersion) {
      showResponse('games/delete', 0, { code: -1, message: '请先刷新列表' });
      return;
    }
    const { httpStatus, envelope } = await client.gamesDelete({
      gameId: (gid.querySelector('input') as HTMLInputElement).value,
      expectedConfigVersion: cachedVersion,
      deleteH5Dir: (delH5 as HTMLInputElement).checked,
    });
    showResponse('games/delete', httpStatus, envelope);
    if (envelope.data?.configVersion) cachedVersion = envelope.data.configVersion;
    if (envelope.code === 0) void doRefresh();
  };

  // —— H5 上传：选择 zip / tar.gz，点「重新上传」发布 ——
  const uploadCard = el('div', 'card');
  uploadCard.append(el('h2', undefined, 'H5 上传 (选择文件)'));
  const packageInput = el('input') as HTMLInputElement;
  packageInput.type = 'file';
  packageInput.accept = '.zip,.tar.gz,application/zip,application/gzip,application/x-gzip';
  const fileSummary = el('p', 'hint', '未选择包文件');
  const metaReloadStatus = el('p', 'hint', '修改 gameId 后自动填充配置；选包后点「重新上传」发布');
  const metaForm = el('div', 'meta-form');
  const gameIdDatalist = el('datalist');
  gameIdDatalist.id = 'h5UploadGameIdList';
  let gameIdInput: HTMLInputElement | null = null;
  for (const f of H5_META_FIELDS) {
    const { row, input } = makeMetaControl(el, f);
    if (f.key === 'gameId') {
      gameIdInput = input as HTMLInputElement;
      gameIdInput.setAttribute('list', 'h5UploadGameIdList');
    }
    metaForm.append(row);
  }
  metaForm.append(gameIdDatalist);

  const ensureRowsLoaded = async (): Promise<IdipGameRow[]> => {
    if (cachedRows.length > 0) return cachedRows;
    await doRefresh();
    return cachedRows;
  };

  if (gameIdInput) {
    wireGameIdMetaReload({
      gameIdInput,
      metaForm,
      getRows: () => cachedRows,
      ensureRowsLoaded,
      onStatus: (message, ok) => {
        metaReloadStatus.className = ok ? 'hint meta-reload-ok' : 'hint meta-reload-fail';
        metaReloadStatus.textContent = message;
      },
    });
  }

  let uploadBusy = false;
  const uploadBtn = el('button', 'primary', '重新上传');
  uploadBtn.disabled = true;

  packageInput.onchange = () => {
    const file = packageInput.files?.[0];
    if (!file) {
      fileSummary.textContent = '未选择包文件';
      uploadBtn.disabled = true;
      return;
    }
    fileSummary.textContent = `已选：${file.name}（${(file.size / 1024 / 1024).toFixed(2)} MB）`;
    uploadBtn.disabled = false;
    const ver = parseMinigameVersionFromPackageFileName(file.name);
    if (ver) {
      setH5MetaFieldValue(metaForm, 'minigameVersion', ver);
    }
  };

  const runH5PackageUpload = async (): Promise<boolean> => {
    if (uploadBusy) return false;
    const file = packageInput.files?.[0];
    if (!file) {
      showResponse('games/h5/upload', 0, { code: -1, message: '请选择 zip 或 tar.gz 包' });
      fileSummary.textContent = '请先选择包文件';
      return false;
    }
    const gameId = gameIdInput?.value.trim() ?? '';
    if (!gameId) {
      showResponse('games/h5/upload', 0, { code: -1, message: '请填写 gameId' });
      return false;
    }
    const meta = collectH5Meta(metaForm);
    if (!meta) {
      showResponse('games/h5/upload', 0, { code: -1, message: '请填写必填 meta 字段（含 contentVersion）' });
      return false;
    }
    meta.gameId = gameId;
    const parsedVer = parseMinigameVersionFromPackageFileName(file.name);
    if (parsedVer) {
      meta.minigameVersion = parsedVer;
    }
    uploadBusy = true;
    uploadBtn.disabled = true;
    fileSummary.textContent = `发布中：${file.name} …`;
    try {
      const { httpStatus, envelope } = await client.gamesH5Upload(file, meta);
      showResponse('games/h5/upload', httpStatus, envelope);
      const data = envelope.data as
        | { configVersion?: string; gameDirName?: string; minigameVersion?: string }
        | undefined;
      if (data?.configVersion) {
        cachedVersion = data.configVersion;
        versionSpan.textContent = `configVersion: ${cachedVersion.slice(0, 16)}…`;
      }
      if (envelope.code === 0) {
        const pkg =
          data?.gameDirName && data?.minigameVersion
            ? `${data.gameDirName}_v${data.minigameVersion}.tar.gz`
            : file.name;
        fileSummary.textContent = `发布成功：release_h5/${pkg} · 已同步 CDN · games.json 已更新`;
        void doRefresh();
        return true;
      }
      fileSummary.textContent = `发布失败：${envelope.message || httpStatus}`;
      return false;
    } finally {
      uploadBusy = false;
      uploadBtn.disabled = !packageInput.files?.[0];
    }
  };

  uploadCard.append(
    deps.wrapLabel('包文件 (.zip / .tar.gz)', packageInput),
    fileSummary,
    metaReloadStatus,
    el(
      'p',
      'hint',
      '流程：zip 须为 {gameDir}.zip，包内顶层目录同名且含 index.html；tar.gz 可为 {gameDir}_v{版本}.tar.gz（内层可为 {gameDir}/ 或 {gameDir}_v{版本}/）。选包后点「重新上传」。',
    ),
    metaForm,
    uploadBtn,
  );

  const syncH5UploadForm = (rows: IdipGameRow[]) => {
    syncH5GameIdDatalist(gameIdDatalist, rows);
  };
  uploadFormSync = syncH5UploadForm;

  uploadBtn.onclick = () => void runH5PackageUpload();

  const pre = el('pre', 'response', '{}');
  wrap.append(listCard, batchCard, delCard, uploadCard, pre);
  return wrap;
}
