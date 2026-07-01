import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { IdipClient } from '../api/idipClient';
import { ensureServerReachable } from './runner';
import { h5UploadMeta, loadH5ZipFixture, VITEST_GAME_DIR } from './publishFixtures';

const baseUrl = (process.env.IDIP_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
const idipKey = process.env.IDIP_KEY ?? 'change-me-in-production';
const username = process.env.IDIP_USERNAME ?? 'holyjing';
const password = process.env.IDIP_PASSWORD ?? 'jgyjgyjgy';

const client = new IdipClient(baseUrl, idipKey);

let serverOk = false;
let skipReason = '';
let sessionToken = '';
let configVersion = '';

beforeAll(async () => {
  const err = await ensureServerReachable(client);
  if (err) {
    skipReason = err;
    return;
  }
  serverOk = true;
  const login = await client.login(username, password);
  if (!login.ok) {
    serverOk = false;
    skipReason = `login failed: ${login.envelope.message}`;
  }
  sessionToken = client.sessionToken;
});

afterAll(async () => {
  if (sessionToken) await client.logout();
});

async function refreshConfigVersion() {
  const list = await client.gamesList();
  configVersion = list.envelope.data?.configVersion ?? configVersion;
  return configVersion;
}

describe('IDIP publish regression (P0–P3)', () => {
  it('server reachable', (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
  });

  it('IDP-AUTH-001 invalid login returns 1401', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const bad = new IdipClient(baseUrl, idipKey);
    const { httpStatus, envelope } = await bad.login('not-a-real-user', 'wrong-password');
    expect(httpStatus).toBe(401);
    expect(envelope.code).toBe(1401);
    expect(bad.sessionToken).toBe('');
  });

  it('IDP-AUTH-002 login session + heartbeat', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    expect(sessionToken).not.toBe('');
    expect(await client.heartbeat()).toBe(true);
  });

  it('GAMES list returns configVersion', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const { httpStatus, envelope } = await client.gamesList();
    expect(httpStatus).toBe(200);
    expect(envelope.code).toBe(0);
    configVersion = envelope.data?.configVersion ?? '';
    expect(configVersion.length).toBeGreaterThan(10);
  });

  it('GAMES-CONF-001 upsert with stale configVersion → 409', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    await refreshConfigVersion();
    const stale = '0'.repeat(64);
    const { httpStatus, envelope } = await client.gamesUpsert({
      expectedConfigVersion: stale,
      gameId: 'g001',
      name: 'conflict probe',
    });
    expect(httpStatus).toBe(409);
    expect(envelope.code).toBe(1409);
  });

  it('GAMES upsert + batch-upsert', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    await refreshConfigVersion();
    const up = await client.gamesUpsert({
      expectedConfigVersion: configVersion,
      gameId: 'g001',
      name: '宝石消消乐',
      status: 'online',
    });
    expect(up.httpStatus).toBe(200);
    expect(up.envelope.code).toBe(0);
    configVersion = up.envelope.data?.configVersion ?? configVersion;
    const batch = await client.gamesBatchUpsert({
      expectedConfigVersion: configVersion,
      items: [{ gameId: 'g001', sort: 1 }],
    });
    expect(batch.httpStatus).toBe(200);
    expect(batch.envelope.code).toBe(0);
    configVersion = batch.envelope.data?.configVersion ?? configVersion;
  });

  it('GAMES-DEL-001 delete online game → 400', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    await refreshConfigVersion();
    const { httpStatus, envelope } = await client.gamesDelete({
      gameId: 'g001',
      expectedConfigVersion: configVersion,
      deleteH5Dir: false,
    });
    expect(httpStatus).toBe(400);
    expect(envelope.code).toBe(1400);
    expect(envelope.message).toMatch(/offline/i);
  });

  it('H5-UP-011 zip name mismatch → 400', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const file = loadH5ZipFixture('vitest-badname.zip');
    const { httpStatus, envelope } = await client.gamesH5Upload(
      file,
      h5UploadMeta({ gameId: `vitest-bad-${Date.now()}` }),
    );
    expect(httpStatus).toBe(400);
    expect(envelope.code).toBe(1400);
  });

  it('H5-UP upload offline package + GAMES-DEL-002 delete json only', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const gameId = `vitest-h5-${Date.now()}`;
    const file = loadH5ZipFixture('vitest-game1.zip');
    const upload = await client.gamesH5Upload(
      file,
      h5UploadMeta({ gameId, minigameVersion: '1.0.0.1', status: 'offline' }),
    );
    expect(upload.httpStatus).toBe(200);
    expect(upload.envelope.code).toBe(0);

    await refreshConfigVersion();
    const del = await client.gamesDelete({
      gameId,
      expectedConfigVersion: configVersion,
      deleteH5Dir: false,
    });
    expect(del.httpStatus).toBe(200);
    expect(del.envelope.code).toBe(0);
    expect(del.envelope.data?.gameId).toBe(gameId);
    expect(del.envelope.data?.h5DirDeleted).toBe(false);
    expect(del.envelope.data?.gameDirName).toBe(VITEST_GAME_DIR);
    configVersion = del.envelope.data?.configVersion ?? configVersion;
  });

  it('AUDIT-001 audit logs pagination + filters', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const loginRows = await client.auditLogs({ limit: 5, offset: 0, action: 'login' });
    expect(loginRows.httpStatus).toBe(200);
    expect(loginRows.envelope.code).toBe(0);
    expect((loginRows.envelope.data?.total ?? 0) >= 1).toBe(true);
    expect(Array.isArray(loginRows.envelope.data?.list)).toBe(true);

    const page = await client.auditLogs({ limit: 1, offset: 0 });
    expect(page.httpStatus).toBe(200);
    expect(page.envelope.code).toBe(0);
    expect((page.envelope.data?.list?.length ?? 0)).toBeLessThanOrEqual(1);
  });

  it('AUDIT-002 games_upsert / h5_upload audit rows', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const upsertAudit = await client.auditLogs({ limit: 20, action: 'games_upsert' });
    expect(upsertAudit.httpStatus).toBe(200);
    expect(upsertAudit.envelope.code).toBe(0);
    expect((upsertAudit.envelope.data?.total ?? 0) >= 1).toBe(true);

    const h5Audit = await client.auditLogs({ limit: 20, action: 'h5_upload' });
    expect(h5Audit.httpStatus).toBe(200);
    expect(h5Audit.envelope.code).toBe(0);
    expect((h5Audit.envelope.data?.total ?? 0) >= 1).toBe(true);
  });

  it('PUBLISH rsync-retry noop when disabled', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const { httpStatus, envelope } = await client.publishRsyncRetry({});
    expect(httpStatus).toBe(200);
    expect(envelope.code).toBe(0);
  });

  it('IDP-AUTH-003 logout invalidates session', async (ctx) => {
    if (!serverOk) ctx.skip(skipReason);
    const tmp = new IdipClient(baseUrl, idipKey);
    const login = await tmp.login(username, password);
    expect(login.ok).toBe(true);
    await tmp.logout();
    expect(tmp.sessionToken).toBe('');
    expect(await tmp.heartbeat()).toBe(false);
    const again = await client.login(username, password);
    expect(again.ok).toBe(true);
    sessionToken = client.sessionToken;
  });
});
