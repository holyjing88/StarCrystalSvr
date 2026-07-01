import type {
  ApiEnvelope,
  GoldApplyResultDto,
  GoldSetUserRequest,
  IdipLoginData,
  MonthTokenPoolData,
  TaskDefinitionsData,
  TaskTierPolicy,
  WelfareUserProgressData,
} from './types';

export type SessionExpiredHandler = () => void;

/**
 * StarCrystal IDIP HTTP 客户端。
 *
 * 运营台：login → X-IDIP-Session + 心跳
 * 脚本/Vitest：可仅 X-IDIP-Key
 *
 * @see ../../../../server-go/doc/IDIP_API.md
 */
export class IdipClient {
  private sessionExpiredHandler: SessionExpiredHandler | null = null;

  constructor(
    public baseUrl: string,
    public idipKey: string,
    public sessionToken: string = '',
  ) {}

  static fromStorage(): IdipClient {
    const baseUrl =
      (typeof localStorage !== 'undefined' ? localStorage.getItem('idip.baseUrl') : null) ?? '';
    const idipKey =
      (typeof localStorage !== 'undefined' ? localStorage.getItem('idip.key') : null) ??
      import.meta.env.VITE_IDIP_KEY ??
      'change-me-in-production';
    const sessionToken =
      (typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('idip.sessionToken') : null) ??
      '';
    return new IdipClient(baseUrl.replace(/\/$/, ''), idipKey, sessionToken);
  }

  saveToStorage(): void {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem('idip.baseUrl', this.baseUrl);
    localStorage.setItem('idip.key', this.idipKey);
    if (typeof sessionStorage === 'undefined') return;
    if (this.sessionToken) {
      sessionStorage.setItem('idip.sessionToken', this.sessionToken);
    } else {
      sessionStorage.removeItem('idip.sessionToken');
    }
  }

  setSessionExpiredHandler(handler: SessionExpiredHandler | null): void {
    this.sessionExpiredHandler = handler;
  }

  async request<T = unknown>(
    method: string,
    path: string,
    options?: {
      body?: unknown;
      query?: Record<string, string>;
      idipKeyOverride?: string;
      omitKey?: boolean;
      omitSession?: boolean;
    },
  ): Promise<{ httpStatus: number; envelope: ApiEnvelope<T> }> {
    let url = `${this.baseUrl}${path}`;
    if (options?.query) {
      const q = new URLSearchParams(options.query);
      url += (url.includes('?') ? '&' : '?') + q.toString();
    }
    const headers: Record<string, string> = {
      Accept: 'application/json',
    };
    const useSession = !options?.omitSession && !!this.sessionToken;
    if (useSession) {
      headers['X-IDIP-Session'] = this.sessionToken;
    } else if (!options?.omitKey) {
      headers['X-IDIP-Key'] = options?.idipKeyOverride ?? this.idipKey;
    }
    let body: string | undefined;
    if (options?.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }
    const res = await fetch(url, { method, headers, body });
    let envelope: ApiEnvelope<T>;
    try {
      envelope = (await res.json()) as ApiEnvelope<T>;
    } catch {
      envelope = { code: -1, message: `non-json response (${res.status})` };
    }
    if (
      res.status === 401 &&
      useSession &&
      !path.includes('/auth/login') &&
      !path.includes('/auth/heartbeat')
    ) {
      this.sessionToken = '';
      if (typeof sessionStorage !== 'undefined') {
        sessionStorage.removeItem('idip.sessionToken');
      }
      this.sessionExpiredHandler?.();
    }
    return { httpStatus: res.status, envelope };
  }

  async login(username: string, password: string) {
    const { httpStatus, envelope } = await this.request<IdipLoginData>(
      'POST',
      '/idip/v1/auth/login',
      { body: { username, password }, omitKey: true, omitSession: true },
    );
    if (httpStatus === 200 && envelope.code === 0 && envelope.data?.sessionToken) {
      this.sessionToken = envelope.data.sessionToken;
      if (typeof sessionStorage !== 'undefined') {
        sessionStorage.setItem('idip.sessionToken', this.sessionToken);
        sessionStorage.setItem('idip.username', username);
        if (envelope.data.heartbeatIntervalSec && envelope.data.heartbeatIntervalSec > 0) {
          sessionStorage.setItem('idip.heartbeatIntervalSec', String(envelope.data.heartbeatIntervalSec));
        }
      }
    }
    return { ok: httpStatus === 200 && envelope.code === 0, httpStatus, envelope };
  }

  async logout(): Promise<void> {
    if (this.sessionToken) {
      await this.request('POST', '/idip/v1/auth/logout', { omitKey: true });
    }
    this.sessionToken = '';
    if (typeof sessionStorage !== 'undefined') {
      sessionStorage.removeItem('idip.sessionToken');
      sessionStorage.removeItem('idip.username');
    }
  }

  async heartbeat(): Promise<boolean> {
    if (!this.sessionToken) return false;
    const { httpStatus, envelope } = await this.request<{ expiresAt?: string }>(
      'POST',
      '/idip/v1/auth/heartbeat',
      { omitKey: true },
    );
    return httpStatus === 200 && envelope.code === 0;
  }

  gamesList() {
    return this.request<import('./types').IdipGamesListData>('GET', '/idip/v1/games/list');
  }

  gamesUpsert(body: {
    expectedConfigVersion: string;
    gameId: string;
    name?: string;
    status?: string;
    sort?: number;
  }) {
    return this.request<{ configVersion: string }>('POST', '/idip/v1/games/upsert', { body });
  }

  gamesDelete(body: {
    gameId: string;
    expectedConfigVersion: string;
    deleteH5Dir?: boolean;
  }) {
    return this.request<{
      gameId: string;
      configVersion: string;
      gameDirName: string;
      h5DirDeleted: boolean;
    }>('POST', '/idip/v1/games/delete', { body });
  }

  gamesBatchUpsert(body: {
    expectedConfigVersion: string;
    items: Array<{
      gameId: string;
      name?: string;
      status?: string;
      sort?: number;
    }>;
  }) {
    return this.request<{ configVersion: string }>('POST', '/idip/v1/games/batch-upsert', {
      body,
    });
  }

  async gamesH5UploadFolder(files: File[], meta: Record<string, unknown>) {
    const form = new FormData();
    form.append('uploadMode', 'folder');
    form.append('meta', JSON.stringify(meta));
    for (const file of files) {
      const rel = file.webkitRelativePath || file.name;
      form.append('files', file, rel);
    }
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (this.sessionToken) {
      headers['X-IDIP-Session'] = this.sessionToken;
    } else {
      headers['X-IDIP-Key'] = this.idipKey;
    }
    const url = `${this.baseUrl}/idip/v1/games/h5/upload`;
    const res = await fetch(url, { method: 'POST', headers, body: form });
    let envelope: ApiEnvelope<unknown>;
    try {
      envelope = (await res.json()) as ApiEnvelope<unknown>;
    } catch {
      envelope = { code: -1, message: `non-json response (${res.status})` };
    }
    return { httpStatus: res.status, envelope };
  }

  async gamesH5Upload(file: File, meta: Record<string, unknown>) {
    const form = new FormData();
    form.append('file', file);
    form.append('meta', JSON.stringify(meta));
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (this.sessionToken) {
      headers['X-IDIP-Session'] = this.sessionToken;
    } else {
      headers['X-IDIP-Key'] = this.idipKey;
    }
    const url = `${this.baseUrl}/idip/v1/games/h5/upload`;
    const res = await fetch(url, { method: 'POST', headers, body: form });
    let envelope: ApiEnvelope<unknown>;
    try {
      envelope = (await res.json()) as ApiEnvelope<unknown>;
    } catch {
      envelope = { code: -1, message: `non-json response (${res.status})` };
    }
    return { httpStatus: res.status, envelope };
  }

  publishRsyncRetry(body?: { kind?: string; gameDirName?: string }) {
    return this.request('POST', '/idip/v1/publish/rsync-retry', { body: body ?? {} });
  }

  auditLogs(query?: { limit?: number; offset?: number; action?: string; gameId?: string }) {
    const q: Record<string, string> = {};
    if (query?.limit != null) q.limit = String(query.limit);
    if (query?.offset != null) q.offset = String(query.offset);
    if (query?.action) q.action = query.action;
    if (query?.gameId) q.gameId = query.gameId;
    return this.request<{ total: number; list: unknown[] }>('GET', '/idip/v1/audit/logs', {
      query: Object.keys(q).length ? q : undefined,
    });
  }

  goldSetUser(body: GoldSetUserRequest) {
    return this.request<GoldApplyResultDto>('POST', '/idip/v1/gold/set-user', { body });
  }

  goldMonthUser(accountId: string, yyyymm?: string) {
    const query: Record<string, string> = { accountId };
    if (yyyymm) query.yyyymm = yyyymm;
    return this.request<{ note?: string; userGoldDelta?: number }>(
      'GET',
      '/idip/v1/gold/month-user',
      { query },
    );
  }

  getMonthTokenPool(yyyymm?: string) {
    const query = yyyymm ? { yyyymm } : undefined;
    return this.request<MonthTokenPoolData>('GET', '/idip/v1/welfare/month-token-pool', {
      query,
    });
  }

  setMonthTokenPool(pool: number, yyyymm?: string) {
    return this.request('POST', '/idip/v1/welfare/set-month-token-pool', {
      body: { pool, yyyymm: yyyymm || undefined },
    });
  }

  runMonthlySettlement(force = false, yyyymm?: string) {
    return this.request('POST', '/idip/v1/welfare/run-monthly-settlement', {
      body: { force, yyyymm: yyyymm || undefined },
    });
  }

  goldRecalcServerDelta() {
    return this.request('POST', '/idip/v1/gold/recalc-server-delta-total', { body: {} });
  }

  taskDefinitions() {
    return this.request<TaskDefinitionsData>('GET', '/idip/v1/tasks/definitions');
  }

  taskTierPolicy(policy: TaskTierPolicy) {
    return this.request<TaskTierPolicy>('POST', '/idip/v1/tasks/tier-policy', { body: policy });
  }

  taskDefinitionUpsert(body: {
    taskId: string;
    enabled?: boolean;
    rewardGold?: number;
    target?: number;
  }) {
    return this.request('POST', '/idip/v1/tasks/definition/upsert', { body });
  }

  taskUserProgress(accountId: string) {
    return this.request<WelfareUserProgressData>('GET', '/idip/v1/tasks/user-progress', {
      query: { accountId },
    });
  }

  async ping(): Promise<boolean> {
    const err = await checkIdipReachable(this);
    return err === null;
  }
}

export async function checkPlayerApiReachable(client: IdipClient): Promise<string | null> {
  try {
    const { httpStatus, envelope } = await client.request('POST', '/api/v1/auth/guest', {
      body: { guestKey: '', deviceId: 'idip-preflight-' + Date.now() },
      omitKey: true,
      omitSession: true,
    });
    if (httpStatus !== 200 || envelope.code !== 0) {
      return `POST /api/v1/auth/guest: http ${httpStatus} code=${envelope.code} ${envelope.message}`;
    }
    return null;
  } catch (e) {
    return `POST /api/v1/auth/guest 不可达: ${e instanceof Error ? e.message : String(e)}`;
  }
}

export async function checkIdipReachable(client: IdipClient): Promise<string | null> {
  try {
    const { httpStatus, envelope } = await client.taskDefinitions();
    if (httpStatus === 403 || httpStatus === 401) {
      return `idip ${httpStatus} — 检查登录态或 Key（code=${envelope.code}）`;
    }
    if (httpStatus !== 200) return `HTTP ${httpStatus}`;
    if (envelope.code !== 0) return `code=${envelope.code} ${envelope.message}`;
    return null;
  } catch (e) {
    return e instanceof Error ? e.message : String(e);
  }
}
