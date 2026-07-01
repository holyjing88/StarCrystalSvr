import { checkIdipReachable, type IdipClient } from '../api/idipClient';
import type { CaseResult } from '../api/types';
import { REGRESSION_CASES, type RegressionCase } from './cases';

export async function runRegression(
  client: IdipClient,
  cases: RegressionCase[] = REGRESSION_CASES,
  onProgress?: (id: string, index: number, total: number) => void,
): Promise<CaseResult[]> {
  const results: CaseResult[] = [];
  for (let i = 0; i < cases.length; i++) {
    const c = cases[i];
    onProgress?.(c.id, i + 1, cases.length);
    const t0 = performance.now();
    try {
      await c.run(client);
      results.push({
        id: c.id,
        name: `${c.api} — ${c.verify}`,
        passed: true,
        durationMs: Math.round(performance.now() - t0),
      });
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      results.push({
        id: c.id,
        name: `${c.api} — ${c.verify}`,
        passed: false,
        durationMs: Math.round(performance.now() - t0),
        error: msg,
      });
    }
  }
  return results;
}

export async function ensureServerReachable(client: IdipClient): Promise<string | null> {
  return checkIdipReachable(client);
}
