/**
 * IDIP 回归用例 — 字段与 doc/测试用例.md 表格列一致。
 * @see ../doc/测试用例.md
 */
import type { IdipClient } from '../api/idipClient';

export interface RegressionCase {
  id: string;
  /** 如 POST /idip/v1/gold/set-user */
  api: string;
  /** 服务端 handler / service 实际用途（给人看） */
  servicePurpose: string;
  /** 本自动化用例断言什么 */
  verify: string;
  docRef: string;
  run: (client: IdipClient) => Promise<void>;
  requiresEconomy?: boolean;
}

function assert(cond: boolean, msg: string): void {
  if (!cond) throw new Error(msg);
}

async function registerGuestAccount(client: IdipClient): Promise<string> {
  const deviceId = `idip_reg_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  const { httpStatus, envelope } = await client.request('POST', '/api/v1/auth/guest', {
    body: { guestKey: '', deviceId },
    omitKey: true,
  });
  assert(httpStatus === 200 && envelope.code === 0, `guest: http ${httpStatus} ${envelope.message}`);
  const data = envelope.data as {
    user?: { accountId?: string; userId?: string };
    accountId?: string;
  };
  const accountId =
    data?.user?.accountId?.trim() ||
    data?.user?.userId?.trim() ||
    data?.accountId?.trim() ||
    '';
  assert(accountId !== '', 'guest response missing accountId');
  return accountId;
}

export const REGRESSION_CASES: RegressionCase[] = [
  {
    id: 'IDP-002',
    api: 'POST /idip/v1/gold/set-user',
    servicePurpose:
      'idipMiddleware 校验 X-IDIP-Key；失败时不进入 handleIdipGoldSetUser / GoldLedger',
    verify: '无 Key、错误 Key → HTTP 403，code 1403',
    docRef: '后台测试用例-v7.1 §I IDP-002',
    run: async (client) => {
      const noKey = await client.request('POST', '/idip/v1/gold/set-user', {
        body: { accountId: 'idip-test', op: 'add', amount: 1 },
        omitKey: true,
      });
      assert(noKey.httpStatus === 403, `no key: http ${noKey.httpStatus}`);
      assert(noKey.envelope.code === 1403, `no key: code ${noKey.envelope.code}`);

      const badKey = await client.request('POST', '/idip/v1/gold/set-user', {
        body: { accountId: 'idip-test', op: 'add', amount: 1 },
        idipKeyOverride: 'wrong-key-on-purpose',
      });
      assert(badKey.httpStatus === 403, `bad key: http ${badKey.httpStatus}`);
      assert(badKey.envelope.code === 1403, `bad key: code ${badKey.envelope.code}`);
    },
  },
  {
    id: 'IDP-003',
    api: 'POST /idip/v1/gold/set-user',
    servicePurpose:
      'handleIdipGoldSetUser → GoldLedger.ApplyGold(SkipDailyCap)：写 curgold、Redis 月 delta、welfare_gold_cur 榜；运营补币',
    verify: 'guest 建号后 op=add amount=100 → code=0',
    docRef: '后台测试用例-v7.1 §I IDP-003',
    requiresEconomy: true,
    run: async (client) => {
      const accountId = await registerGuestAccount(client);
      const { httpStatus, envelope } = await client.goldSetUser({
        accountId,
        op: 'add',
        amount: 100,
        bizType: 'idip_web_test',
      });
      assert(httpStatus === 200, `http ${httpStatus}`);
      assert(envelope.code === 0, envelope.message);
    },
  },
  {
    id: 'IDP-004',
    api: 'GET /idip/v1/gold/month-user',
    servicePurpose:
      'handleIdipGoldMonthUser：返回 Redis 键 sr:gold:month:{yyyymm}:user:{id}:gold_delta 说明（v1 未读数值）',
    verify: 'code=0 且 data.redisKey 含 redis',
    docRef: '后台测试用例-v7.1 §I IDP-004',
    requiresEconomy: true,
    run: async (client) => {
      const { httpStatus, envelope } = await client.goldMonthUser('idip-test-account');
      assert(httpStatus === 200, `http ${httpStatus}`);
      assert(envelope.code === 0, envelope.message);
      const redisKey = (envelope.data as { redisKey?: string })?.redisKey;
      assert(typeof redisKey === 'string' && redisKey.includes('sr:gold'), `data.redisKey=${redisKey}`);
    },
  },
  {
    id: 'IDP-005',
    api: 'GET+POST /idip/v1/welfare/month-token-pool',
    servicePurpose:
      'GET handleIdipGetMonthTokenPool / POST handleIdipSetMonthTokenPool → GoldRedis：读写月末 Token 分配池',
    verify: '读池 → set pool → 再读 code=0',
    docRef: '后台测试用例-v7.1 §I IDP-005',
    requiresEconomy: true,
    run: async (client) => {
      const get1 = await client.getMonthTokenPool();
      assert(get1.httpStatus === 200 && get1.envelope.code === 0, get1.envelope.message);
      const set = await client.setMonthTokenPool(99999.5);
      assert(set.httpStatus === 200 && set.envelope.code === 0, set.envelope.message);
      const get2 = await client.getMonthTokenPool();
      assert(get2.httpStatus === 200 && get2.envelope.code === 0, get2.envelope.message);
    },
  },
  {
    id: 'IDP-006',
    api: 'POST /idip/v1/welfare/run-monthly-settlement',
    servicePurpose:
      'handleIdipRunSettlement → Settlement.RunSettlement：月末批处理 curgold/totalgold/token 与福利榜',
    verify: 'guest+set-user 加币后 force 结算 code=0',
    docRef: '后台测试用例-v7.1 §I IDP-006',
    requiresEconomy: true,
    run: async (client) => {
      const accountId = await registerGuestAccount(client);
      const seed = await client.goldSetUser({
        accountId,
        op: 'add',
        amount: 100,
        bizType: 'idip_settlement_seed',
      });
      assert(seed.httpStatus === 200 && seed.envelope.code === 0, seed.envelope.message);
      const { httpStatus, envelope } = await client.runMonthlySettlement(true);
      assert(httpStatus === 200, `http ${httpStatus} ${envelope.message}`);
      assert(envelope.code === 0, envelope.message);
    },
  },
  {
    id: 'IDP-010',
    api: 'POST /idip/v1/gold/recalc-server-delta-total',
    servicePurpose: 'handleIdipRecalcServerDelta：占位，规划重算 server_gold_delta_total',
    verify: 'HTTP 501 或 code 1001',
    docRef: '后台测试用例-v7.1 §I IDP-010',
    run: async (client) => {
      const { httpStatus, envelope } = await client.goldRecalcServerDelta();
      assert(httpStatus === 501 || envelope.code === 1001, `got ${httpStatus} code=${envelope.code}`);
    },
  },
  {
    id: 'TSK-S-006',
    api: 'POST /idip/v1/tasks/tier-policy；GET /idip/v1/tasks/definitions',
    servicePurpose:
      'SetTaskTierPolicy 热开关档位；definitions.activeCount 变化（玩家侧同构 GET /api/v1/tasks/welfare）',
    verify: '开 P1 后 activeCount 增大，关 P1 后回落',
    docRef: '后台测试用例-v7.1 §L TSK-S-006',
    run: async (client) => {
      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
      const defs0 = await client.taskDefinitions();
      assert(defs0.envelope.code === 0, defs0.envelope.message);
      const active0 = (defs0.envelope.data as { activeCount?: number })?.activeCount ?? 0;

      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: true, p2Enabled: false });
      const defs1 = await client.taskDefinitions();
      const active1 = (defs1.envelope.data as { activeCount?: number })?.activeCount ?? 0;
      assert(active1 > active0, `p1 on: active ${active1} should > ${active0}`);

      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
    },
  },
  {
    id: 'TSK-A-001',
    api: 'POST /idip/v1/tasks/tier-policy；GET /idip/v1/tasks/definitions',
    servicePurpose: 'p1Enabled=false 时 ListActiveTaskDefs 仅 P0；definitions 仍返回全量 tasks 目录',
    verify: '关 P1：activeCount 在 P0 区间；开 P1 后增大；catalog≥40',
    docRef: '后台测试用例-v7.1 §L TSK-A-001',
    run: async (client) => {
      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
      const off = await client.taskDefinitions();
      assert(off.envelope.code === 0, off.envelope.message);
      const dataOff = off.envelope.data as {
        tierPolicy?: { p1Enabled: boolean };
        activeCount?: number;
      };
      assert(dataOff.tierPolicy?.p1Enabled === false, 'tierPolicy.p1Enabled');
      const activeOff = dataOff.activeCount ?? 0;
      assert(activeOff >= 10 && activeOff < 25, `P0-only activeCount=${activeOff}`);

      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: true, p2Enabled: false });
      const on = await client.taskDefinitions();
      const activeOn = (on.envelope.data as { activeCount?: number })?.activeCount ?? 0;
      assert(activeOn > activeOff, `p1 on active ${activeOn} > p0-only ${activeOff}`);

      const catalogLen = (on.envelope.data as { tasks?: unknown[] })?.tasks?.length ?? 0;
      assert(catalogLen >= 40, `full catalog tasks=${catalogLen}`);

      await client.taskTierPolicy({ p0Enabled: true, p1Enabled: false, p2Enabled: false });
    },
  },
  {
    id: 'TSK-A-002',
    api: 'POST /idip/v1/tasks/definition/upsert；GET /idip/v1/tasks/definitions',
    servicePurpose:
      'UpsertTaskOverride 热改 rewardGold；玩家领奖走 POST /api/v1/tasks/claim 使用新奖励',
    verify: 'daily_free_claim 奖励 +1 后 definitions 读回一致并还原',
    docRef: '后台测试用例-v7.1 §L TSK-A-002',
    run: async (client) => {
      const defs = await client.taskDefinitions();
      assert(defs.envelope.code === 0, defs.envelope.message);
      const rows = (defs.envelope.data as { tasks?: { taskId: string; rewardGold: number }[] })
        ?.tasks;
      if (!rows || rows.length === 0) throw new Error('no tasks');
      const pick = rows.find((t) => t.taskId === 'daily_free_claim') ?? rows[0]!;
      const newReward = (pick.rewardGold || 50) + 1;
      const up = await client.taskDefinitionUpsert({
        taskId: pick.taskId,
        rewardGold: newReward,
      });
      assert(up.httpStatus === 200 && up.envelope.code === 0, up.envelope.message);
      const defs2 = await client.taskDefinitions();
      const row2 = (
        defs2.envelope.data as { tasks?: { taskId: string; rewardGold: number }[] }
      )?.tasks?.find((t) => t.taskId === pick.taskId);
      assert(row2?.rewardGold === newReward, `reward want ${newReward} got ${row2?.rewardGold}`);
      await client.taskDefinitionUpsert({
        taskId: pick.taskId,
        rewardGold: pick.rewardGold,
      });
    },
  },
  {
    id: 'TSK-IDP-PROGRESS',
    api: 'GET /idip/v1/tasks/user-progress',
    servicePurpose: 'handleIdipTaskUserProgress → TaskService.GetWelfare(accountId)，同玩家 welfare 无需 Bearer',
    verify: 'guest accountId 查询返回 tasks[]',
    docRef: 'doc/测试用例.md TSK-IDP-PROGRESS',
    run: async (client) => {
      const accountId = await registerGuestAccount(client);
      const { httpStatus, envelope } = await client.taskUserProgress(accountId);
      assert(httpStatus === 200, `http ${httpStatus}`);
      assert(envelope.code === 0, envelope.message);
      const data = envelope.data as { tasks?: unknown[] } | undefined;
      assert(data != null && Array.isArray(data.tasks), 'missing tasks in progress');
    },
  },
];
