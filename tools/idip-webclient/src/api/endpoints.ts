/**
 * IDIP 路由与用例对照（与 doc/测试用例.md、doc/API与用例对照.md 同步）
 * @see ../../../../server-go/doc/IDIP_API.md
 */

export type IdipEndpointStatus = 'implemented' | 'partial' | 'planned';

export interface IdipEndpointMeta {
  method: 'GET' | 'POST';
  path: string;
  docId: string;
  /** 在 server-go 里实际干什么（handler → service） */
  servicePurpose: string;
  status: IdipEndpointStatus;
  uiTab: 'gold' | 'welfare' | 'tasks' | 'none';
  alias?: boolean;
}

export const IDIP_ENDPOINTS: readonly IdipEndpointMeta[] = [
  {
    method: 'POST',
    path: '/idip/v1/gold/set-user',
    docId: 'IDP-003',
    servicePurpose:
      'handleIdipGoldSetUser → GoldLedger.ApplyGold(SkipDailyCap)：运营改 curgold、写 Redis 月 delta、更新 welfare_gold_cur 榜',
    status: 'implemented',
    uiTab: 'gold',
  },
  {
    method: 'GET',
    path: '/idip/v1/gold/month-user',
    docId: 'IDP-004',
    servicePurpose:
      'handleIdipGoldMonthUser：返回用户当月 gold_delta 的 Redis 键名（v1 未读数值，对账用）',
    status: 'partial',
    uiTab: 'gold',
  },
  {
    method: 'POST',
    path: '/idip/v1/gold/recalc-server-delta-total',
    docId: 'IDP-010',
    servicePurpose: 'handleIdipRecalcServerDelta：未实现；规划重算全服 server_gold_delta_total',
    status: 'partial',
    uiTab: 'gold',
  },
  {
    method: 'POST',
    path: '/idip/v1/gold/recalc-total',
    docId: 'IDP-010',
    servicePurpose: '同上 handler 别名',
    status: 'partial',
    uiTab: 'gold',
    alias: true,
  },
  {
    method: 'GET',
    path: '/idip/v1/welfare/month-token-pool',
    docId: 'IDP-005',
    servicePurpose: 'handleIdipGetMonthTokenPool → GoldRedis：读当月 Token 兑换池（月末 Settlement 用）',
    status: 'implemented',
    uiTab: 'welfare',
  },
  {
    method: 'POST',
    path: '/idip/v1/welfare/set-month-token-pool',
    docId: 'IDP-005',
    servicePurpose: 'handleIdipSetMonthTokenPool → GoldRedis：运营写入当月 Token 池',
    status: 'implemented',
    uiTab: 'welfare',
  },
  {
    method: 'POST',
    path: '/idip/v1/welfare/run-monthly-settlement',
    docId: 'IDP-006',
    servicePurpose: 'handleIdipRunSettlement → Settlement.RunSettlement：触发月末批处理',
    status: 'implemented',
    uiTab: 'welfare',
  },
  {
    method: 'GET',
    path: '/idip/v1/tasks/definitions',
    docId: 'TSK-A-001',
    servicePurpose:
      'handleIdipTaskDefinitions：全量任务表 + tierPolicy + activeCount（ListActiveTaskDefs）',
    status: 'implemented',
    uiTab: 'tasks',
  },
  {
    method: 'POST',
    path: '/idip/v1/tasks/tier-policy',
    docId: 'TSK-S-006',
    servicePurpose:
      'handleIdipTaskTierPolicy → SetTaskTierPolicy：热开关 P0/P1/P2，影响玩家 GET /api/v1/tasks/welfare 条数',
    status: 'implemented',
    uiTab: 'tasks',
  },
  {
    method: 'POST',
    path: '/idip/v1/tasks/definition/upsert',
    docId: 'TSK-A-002',
    servicePurpose:
      'handleIdipTaskDefinitionUpsert → UpsertTaskOverride：热改单任务奖励；玩家 claim 用新额',
    status: 'implemented',
    uiTab: 'tasks',
  },
  {
    method: 'GET',
    path: '/idip/v1/tasks/user-progress',
    docId: 'TSK-IDP-PROGRESS',
    servicePurpose:
      'handleIdipTaskUserProgress → TaskService.GetWelfare(accountId)，同玩家 welfare 无需 Bearer',
    status: 'implemented',
    uiTab: 'tasks',
  },
];
