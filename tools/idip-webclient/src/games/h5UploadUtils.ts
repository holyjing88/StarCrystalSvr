import type { IdipGameRow } from '../api/types';

/** 从 webkitdirectory 选中的文件列表解析唯一顶层目录名（gameDirName）。 */
export function detectH5FolderRootName(files: FileList | File[]): string | null {
  const tops = new Set<string>();
  for (const file of files) {
    const rel = (file.webkitRelativePath || file.name).replace(/\\/g, '/').replace(/^\.\//, '');
    const top = rel.split('/')[0]?.trim();
    if (!top) return null;
    tops.add(top);
  }
  return tops.size === 1 ? [...tops][0] : null;
}

export function syncH5GameIdDatalist(datalist: HTMLDataListElement, rows: IdipGameRow[]): void {
  datalist.replaceChildren();
  for (const row of rows) {
    datalist.append(new Option(`${row.gameId} · ${row.name}`, row.gameId));
  }
}

/** gameId 变更（input / 回车 / 失焦）时从 games 列表填充 meta 表单。 */
export function wireGameIdMetaReload(options: {
  gameIdInput: HTMLInputElement;
  metaForm: HTMLElement;
  getRows: () => IdipGameRow[];
  ensureRowsLoaded: () => Promise<IdipGameRow[]>;
  onStatus?: (message: string, ok: boolean) => void;
}): void {
  let loading = false;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  const tryReload = async () => {
    const gameId = options.gameIdInput.value.trim();
    if (!gameId || loading) return;

    let rows = options.getRows();
    if (rows.length === 0) {
      loading = true;
      options.onStatus?.('正在加载 games 列表…', true);
      try {
        rows = await options.ensureRowsLoaded();
      } finally {
        loading = false;
      }
    }

    const row = findGameRowById(rows, gameId);
    if (row) {
      applyGameRowToH5MetaForm(options.metaForm, row, {
        fillCurrentVersion: true,
        bumpVersion: true,
      });
      options.onStatus?.(`已加载 ${row.gameId} · ${row.name}`, true);
    } else {
      options.onStatus?.(`未找到 ${gameId}，请先刷新列表或检查 gameId`, false);
    }
  };

  const scheduleReload = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void tryReload(), 250);
  };

  options.gameIdInput.addEventListener('input', scheduleReload);
  options.gameIdInput.addEventListener('change', () => void tryReload());
  options.gameIdInput.addEventListener('blur', () => void tryReload());
  options.gameIdInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (debounceTimer) clearTimeout(debounceTimer);
      void tryReload();
    }
  });
}
/** 从包文件名解析四段 minigameVersion（contentVersion），如 `gameDir_v1.0.0.2.tar.gz`。 */
export function parseMinigameVersionFromPackageFileName(fileName: string): string | null {
  const base = fileName.replace(/\\/g, '/').split('/').pop() ?? fileName;
  const m = base.match(/_v(\d+\.\d+\.\d+\.\d+)\.(tar\.gz|zip)$/i);
  return m ? m[1] : null;
}

/** 从 entryUrl 解析 gameDirName（h5/{dir}/index.html）。 */
export function parseGameDirFromEntryUrl(entryUrl: string): string | null {
  const raw = entryUrl.trim();
  if (!raw) return null;
  const m = raw.match(/h5\/([^/?#]+)\/index\.html/i);
  return m ? m[1] : null;
}

export function findGameRowById(rows: IdipGameRow[], gameId: string): IdipGameRow | undefined {
  const id = gameId.trim().toLowerCase();
  if (!id) return undefined;
  return rows.find((r) => r.gameId.toLowerCase() === id);
}

export function findGameRowByDirName(rows: IdipGameRow[], dirName: string): IdipGameRow | undefined {
  const dir = dirName.trim().toLowerCase();
  if (!dir) return undefined;
  return rows.find((r) => {
    const d = parseGameDirFromEntryUrl(r.entryUrl);
    return d != null && d.toLowerCase() === dir;
  });
}

/** 四段版号递增构建位（末段 +1），用于上传须大于当前版。 */
export function suggestNextMinigameVersion(current: string): string | null {
  const parts = current.trim().split('.');
  if (parts.length !== 4 || parts.some((p) => !/^\d+$/.test(p))) return null;
  const nums = parts.map((p) => Number(p));
  nums[3] += 1;
  return nums.join('.');
}

/** 从 entryUrl 的 `v=` 查询参数解析当前线上版本。 */
export function parseMinigameVersionFromEntryUrl(entryUrl: string): string | null {
  const raw = entryUrl.trim();
  if (!raw) return null;
  try {
    const u = new URL(raw.includes('://') ? raw : `http://local/${raw.replace(/^\//, '')}`);
    const v = u.searchParams.get('v')?.trim();
    return v && /^\d+\.\d+\.\d+\.\d+$/.test(v) ? v : null;
  } catch {
    const m = raw.match(/[?&]v=(\d+\.\d+\.\d+\.\d+)/);
    return m ? m[1] : null;
  }
}

function formatChannels(channels: IdipGameRow['channels']): string {  if (channels == null) return '';
  if (Array.isArray(channels)) return channels.join(', ');
  return String(channels);
}

export type H5MetaFieldKey =
  | 'gameId'
  | 'minigameVersion'
  | 'name'
  | 'nameEn'
  | 'nameUr'
  | 'note'
  | 'noteEn'
  | 'noteUr'
  | 'entryType'
  | 'status'
  | 'sort'
  | 'channels'
  | 'iconLink'
  | 'coverUrl'
  | 'minAppVersion';

const ROW_TO_META: Array<{ metaKey: H5MetaFieldKey; rowKey: keyof IdipGameRow }> = [
  { metaKey: 'name', rowKey: 'name' },
  { metaKey: 'nameEn', rowKey: 'nameEn' },
  { metaKey: 'nameUr', rowKey: 'nameUr' },
  { metaKey: 'entryType', rowKey: 'entryType' },
  { metaKey: 'status', rowKey: 'status' },
  { metaKey: 'sort', rowKey: 'sort' },
  { metaKey: 'iconLink', rowKey: 'iconLink' },
  { metaKey: 'coverUrl', rowKey: 'coverUrl' },
  { metaKey: 'minAppVersion', rowKey: 'minAppVersion' },
];

/** 将 games.json 中已有条目填入 H5 上传 meta 表单（不含 gameId / minigameVersion）。 */
export function applyGameRowToH5MetaForm(
  form: HTMLElement,
  row: IdipGameRow,
  options?: { fillCurrentVersion?: boolean; bumpVersion?: boolean },
): void {  for (const { metaKey, rowKey } of ROW_TO_META) {
    const ctrl = form.querySelector(`[data-meta-key="${metaKey}"]`) as
      | HTMLInputElement
      | HTMLSelectElement
      | null;
    if (!ctrl) continue;
    const raw = row[rowKey];
    if (metaKey === 'sort') {
      ctrl.value = raw != null ? String(raw) : '';
      continue;
    }
    if (metaKey === 'channels') {
      ctrl.value = formatChannels(row.channels);
      continue;
    }
    ctrl.value = raw != null ? String(raw) : '';
  }

  for (const noteKey of ['note', 'noteEn', 'noteUr'] as const) {
    const ctrl = form.querySelector(`[data-meta-key="${noteKey}"]`) as HTMLInputElement | null;
    if (!ctrl) continue;
    const raw = (row as unknown as Record<string, unknown>)[noteKey];
    ctrl.value = raw != null ? String(raw) : '';
  }

  if (options?.fillCurrentVersion) {
    const verCtrl = form.querySelector('[data-meta-key="minigameVersion"]') as HTMLInputElement | null;
    const ver = parseMinigameVersionFromEntryUrl(row.entryUrl);
    if (verCtrl && ver) {
      const next = options.bumpVersion ? suggestNextMinigameVersion(ver) : null;
      verCtrl.value = next ?? ver;
    }
  }
}
export function setH5MetaFieldValue(form: HTMLElement, key: H5MetaFieldKey, value: string): void {
  const ctrl = form.querySelector(`[data-meta-key="${key}"]`) as
    | HTMLInputElement
    | HTMLSelectElement
    | null;
  if (ctrl) ctrl.value = value;
}
