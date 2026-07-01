import type { RegressionCatalogCase, RegressionRowResult } from './catalogTypes';
import {
  formatClientCaseProgressLine,
  type ClientRegressionProgress,
} from './parseUnityUtpLog';

const CLIENT_PHASE_LABEL: Record<string, string> = {
  starting: '启动',
  closing: '结束占用工程的 Unity',
  waiting_unlock: '等待工程锁释放',
  preflight: '检查 server-go',
  editmode: 'Unity EditMode 测试',
  playmode: 'Unity PlayMode 测试',
  parsing: '解析结果',
  done: '完成',
  failed: '结束（有失败）',
  error: '异常',
};

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

type ClientJobResponse =
  | { status: 'idle' }
  | {
      status: 'running';
      phase: string;
      log?: string;
      startedAt?: number;
      progress?: ClientRegressionProgress;
    }
  | {
      status: 'done';
      phase?: string;
      log?: string;
      progress?: ClientRegressionProgress;
      data: { results: RegressionRowResult[]; log?: string; error?: string };
    }
  | {
      status: 'error';
      phase?: string;
      log?: string;
      progress?: ClientRegressionProgress;
      error: string;
      data?: { results: RegressionRowResult[] };
    };

export async function runServerRegressionRemote(
  catalog: RegressionCatalogCase[],
): Promise<{ results: RegressionRowResult[]; log?: string; error?: string }> {
  const res = await fetch('/dev/regression/server', { method: 'POST' });
  const body = (await res.json()) as {
    results?: RegressionRowResult[];
    log?: string;
    error?: string;
  };
  if (!res.ok) {
    return {
      results: catalog.map((c) => ({
        id: c.id,
        passed: false,
        error: body.error ?? `HTTP ${res.status}`,
      })),
      error: body.error,
      log: body.log,
    };
  }
  return { results: body.results ?? [], log: body.log, error: body.error };
}

export async function runClientRegressionRemote(
  catalog: RegressionCatalogCase[],
  onProgress?: (label: string, logTail?: string, progress?: ClientRegressionProgress) => void,
): Promise<{ results: RegressionRowResult[]; log?: string; error?: string }> {
  const browserLoggedCaseIds = new Set<string>();

  const emitBrowserProgress = (progress?: ClientRegressionProgress) => {
    if (!progress) return;
    for (const c of progress.cases) {
      if (c.status === 'pending' || browserLoggedCaseIds.has(c.id)) continue;
      browserLoggedCaseIds.add(c.id);
      console.log(formatClientCaseProgressLine(browserLoggedCaseIds.size, progress.total, c));
    }
  };
  const startRes = await fetch('/dev/regression/client', { method: 'POST' });
  const startBody = (await startRes.json()) as {
    started?: boolean;
    joined?: boolean;
    error?: string;
    job?: ClientJobResponse;
  };

  const shouldPoll =
    startBody.started === true ||
    startBody.joined === true ||
    startBody.job?.status === 'running';

  if (!shouldPoll) {
    const err = startBody.error ?? `无法启动客户端回归（HTTP ${startRes.status}）`;
    onProgress?.(err);
    return {
      results: catalog.map((c) => ({ id: c.id, passed: false, error: err })),
      error: err,
    };
  }

  onProgress?.(
    startBody.joined ? '接续进行中的任务' : '已提交',
    startBody.job && startBody.job.status !== 'idle' ? startBody.job.log : undefined,
  );

  const pollMs = 2000;
  const maxWaitMs = 52 * 60 * 1000;
  const deadline = Date.now() + maxWaitMs;

  while (Date.now() < deadline) {
    await sleep(pollMs);
    let job: ClientJobResponse;
    try {
      const stRes = await fetch('/dev/regression/client/status');
      if (!stRes.ok) {
        onProgress?.(`轮询失败 HTTP ${stRes.status}`);
        continue;
      }
      job = (await stRes.json()) as ClientJobResponse;
    } catch (e) {
      onProgress?.(`轮询异常: ${e instanceof Error ? e.message : String(e)}`);
      continue;
    }

    if (job.status === 'running') {
      const label = CLIENT_PHASE_LABEL[job.phase] ?? job.phase;
      const startJob =
        startBody.job && startBody.job.status === 'running' ? startBody.job : undefined;
      const started = job.startedAt ?? startJob?.startedAt;
      const elapsed =
        started != null ? ` · ${Math.floor((Date.now() - started) / 60000)} 分钟` : '';
      const prog = job.progress;
      const progSuffix = prog ? ` (${prog.completed}/${prog.total})` : '';
      emitBrowserProgress(prog);
      onProgress?.(`${label}${progSuffix}${elapsed}`, job.log, prog);
      continue;
    }

    if (job.status === 'done' && job.data) {
      emitBrowserProgress(job.progress);
      onProgress?.('完成', job.log ?? job.data.log, job.progress);
      return {
        results: job.data.results ?? [],
        log: job.data.log ?? job.log,
        error: job.data.error,
      };
    }

    if (job.status === 'error') {
      const err = job.error ?? '客户端回归失败';
      emitBrowserProgress(job.progress);
      onProgress?.(err, job.log, job.progress);
      return {
        results:
          job.data?.results ??
          catalog.map((c) => ({ id: c.id, passed: false, error: err })),
        error: err,
        log: job.log,
      };
    }

    if (job.status === 'idle') {
      return {
        results: catalog.map((c) => ({
          id: c.id,
          passed: false,
          error: '任务已结束但未返回结果（请重启 npm run dev 后重试）',
        })),
        error: '客户端回归状态异常',
      };
    }
  }

  return {
    results: catalog.map((c) => ({
      id: c.id,
      passed: false,
      error: '轮询超时（>52 分钟）',
    })),
    error: '客户端回归轮询超时',
  };
}

/** 页面打开时若后台仍有任务或已结束未展示，返回 true 表示需要接续轮询/填表 */
export async function fetchClientRegressionJob(): Promise<ClientJobResponse> {
  const stRes = await fetch('/dev/regression/client/status');
  return (await stRes.json()) as ClientJobResponse;
}
