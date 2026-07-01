import type { RegressionCatalogCase } from './catalogTypes';

export type ClientCaseProgressStatus = 'pending' | 'pass' | 'fail' | 'skip';

export interface ClientCaseProgress {
  id: string;
  status: ClientCaseProgressStatus;
  durationMs?: number;
  unityTest?: string;
}

export interface ClientRegressionProgress {
  completed: number;
  total: number;
  cases: ClientCaseProgress[];
}

interface UtpTestEnd {
  fullName: string;
  method: string;
  state: string;
  durationMs?: number;
}

/** 解析 Unity 日志中的 ##utp JSON（需 batch 加 -automated -runTests） */
export function parseUtpTestEndsFromLog(logText: string): UtpTestEnd[] {
  const out: UtpTestEnd[] = [];
  for (const line of logText.split(/\r?\n/)) {
    const marker = '##utp:';
    const idx = line.indexOf(marker);
    if (idx < 0) continue;
    try {
      const msg = JSON.parse(line.slice(idx + marker.length)) as {
        type?: string;
        phase?: string;
        name?: string;
        state?: string | number;
        duration?: number;
      };
      if (msg.type !== 'TestStatus' || msg.phase !== 'End' || !msg.name) continue;
      const state = utpStateLabel(msg.state);
      out.push({
        fullName: msg.name,
        method: msg.name.split('.').pop() ?? msg.name,
        state,
        durationMs: typeof msg.duration === 'number' ? msg.duration : undefined,
      });
    } catch {
      /* ignore malformed utp line */
    }
  }
  return out;
}

function utpStateLabel(state: string | number | undefined): string {
  if (state === 4 || state === 'Success') return 'Success';
  if (state === 2 || state === 'Skipped') return 'Skipped';
  if (state === 3 || state === 'Ignored') return 'Ignored';
  if (state === 5 || state === 'Failure') return 'Failure';
  if (state === 6 || state === 'Error') return 'Error';
  if (state === 0 || state === 'Inconclusive') return 'Inconclusive';
  return String(state ?? 'Unknown');
}

function utpToCaseStatus(state: string): ClientCaseProgressStatus {
  if (state === 'Success') return 'pass';
  if (state === 'Skipped' || state === 'Ignored' || state === 'Inconclusive') return 'skip';
  return 'fail';
}

/** 将 UTP 结束事件映射到回归目录（按 unityTest 方法名） */
export function buildClientProgressFromUtp(
  catalog: RegressionCatalogCase[],
  utpEnds: UtpTestEnd[],
): ClientRegressionProgress {
  const byMethod = new Map<string, UtpTestEnd>();
  for (const e of utpEnds) {
    byMethod.set(e.method, e);
  }

  const cases: ClientCaseProgress[] = catalog.map((c) => {
    const hit = c.unityTest ? byMethod.get(c.unityTest) : undefined;
    if (!hit) {
      return { id: c.id, status: 'pending', unityTest: c.unityTest };
    }
    return {
      id: c.id,
      status: utpToCaseStatus(hit.state),
      durationMs: hit.durationMs,
      unityTest: c.unityTest,
    };
  });

  const completed = cases.filter((x) => x.status !== 'pending').length;
  return { completed, total: cases.length, cases };
}

export function formatClientCaseProgressLine(
  index: number,
  total: number,
  c: ClientCaseProgress,
): string {
  const tag =
    c.status === 'pass' ? 'PASS' : c.status === 'skip' ? 'SKIP' : c.status === 'fail' ? 'FAIL' : 'PEND';
  const ms = c.durationMs != null ? ` ${c.durationMs}ms` : '';
  return `[client-regression] ${index}/${total} ${c.id} ${tag}${ms}`;
}
