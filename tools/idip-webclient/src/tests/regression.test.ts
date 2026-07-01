import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { IdipClient } from '../api/idipClient';

function idipClientFromEnv(): IdipClient {
  const baseUrl = (process.env.IDIP_BASE_URL ?? 'http://127.0.0.1:8080').replace(
    /\/$/,
    '',
  );
  const idipKey = process.env.IDIP_KEY ?? 'change-me-in-production';
  return new IdipClient(baseUrl, idipKey);
}
import { REGRESSION_CASES } from './cases';
import { ensureServerReachable, runRegression } from './runner';

const client = idipClientFromEnv();
let serverOk = false;
let skipReason = '';

beforeAll(async () => {
  const err = await ensureServerReachable(client);
  if (err) {
    skipReason = err;
    return;
  }
  serverOk = true;
  await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
});

afterAll(async () => {
  if (serverOk) {
    await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
  }
});

describe(`IDIP regression (${(process.env.IDIP_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '')})`, () => {
  it('server is reachable', (ctx) => {
    if (!serverOk) ctx.skip(skipReason || 'start server-go on 127.0.0.1:8080');
  });

  for (const c of REGRESSION_CASES) {
    it(`${c.id} ${c.api}`, async (ctx) => {
      if (!serverOk) ctx.skip(skipReason);
      const results = await runRegression(client, [c]);
      const r = results[0]!;
      expect(r.passed, r.error).toBe(true);
    });
  }
});
