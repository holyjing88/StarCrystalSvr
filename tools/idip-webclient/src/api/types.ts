/**
 * IDIP 与 server-go `api.Response` 对齐的类型定义。
 * @see ../../../../server-go/doc/IDIP_API.md §0.2
 */

/** 统一 JSON 信封：code=0 表示业务成功 */
export interface ApiEnvelope<T = unknown> {
  /** 业务码，0 成功；1403 IDIP 鉴权失败；1501 服务未配置等 */
  code: number;
  message: string;
  data?: T;
}

export interface IdipConfig {
  /** API 根 URL，开发环境留空走 Vite `/idip` 代理 */
  baseUrl: string;
  /** 请求头 X-IDIP-Key，对应 starcrystal.json → idip.key（Vitest/脚本） */
  idipKey: string;
}

export interface IdipLoginData {
  sessionToken: string;
  expiresAt: string;
  heartbeatIntervalSec?: number;
}

export interface IdipGameRow {
  gameId: string;
  name: string;
  nameEn?: string;
  nameUr?: string;
  note?: string;
  noteEn?: string;
  noteUr?: string;
  entryType: string;
  entryUrl: string;
  sort: number;
  status?: string;
  missingVersion?: boolean;
  channels?: string | string[];
  iconLink?: string;
  coverUrl?: string;
  minAppVersion?: string;
  downloadUrl?: string;
  packageBytes?: number;
  downloadSha256?: string;
}

export interface IdipGamesListData {
  list: IdipGameRow[];
  configVersion: string;
  configPath: string;
}

/** POST /idip/v1/gold/set-user */
export interface GoldSetUserRequest {
  accountId: string;
  op: 'add' | 'deduct' | 'set';
  amount: number;
  bizType?: string;
}

/** GoldLedger ApplyGold 成功返回（字段名与 Go json tag 一致） */
export interface GoldApplyResultDto {
  before?: EconomyBalancesDto;
  after?: EconomyBalancesDto;
  requestedDelta?: number;
  grantedDelta?: number;
  dailyCapRemaining?: number;
}

export interface EconomyBalancesDto {
  curgold?: number;
  totalgold?: number;
  curtoken?: number;
  totaltoken?: number;
}

/** GET month-token-pool */
export interface MonthTokenPoolData {
  yyyymm?: string;
  monthTokenPool?: number;
  from?: 'json' | string;
}

/** POST/GET tier-policy */
export interface TaskTierPolicy {
  p0Enabled: boolean;
  p1Enabled: boolean;
  p2Enabled: boolean;
}

export interface TaskDefinitionRow {
  taskId: string;
  tier: string;
  category: string;
  enabled: boolean;
  target: number;
  rewardGold: number;
  metric: string;
}

/** GET /idip/v1/tasks/definitions */
export interface TaskDefinitionsData {
  tierPolicy: TaskTierPolicy;
  tasks: TaskDefinitionRow[];
  activeCount: number;
}

/** GET user-progress ≈ GET /api/v1/tasks/welfare（见 PLAYER_TASK_API.md） */
export interface WelfareUserProgressData {
  todayYmd?: number;
  tierPolicy?: TaskTierPolicy;
  signin7d?: unknown;
  tasks?: WelfareTaskItemDto[];
}

export interface WelfareTaskItemDto {
  taskId: string;
  status: string;
  progress?: number;
  target?: number;
  rewardGold?: number;
}

export interface CaseResult {
  id: string;
  name: string;
  passed: boolean;
  durationMs: number;
  error?: string;
  httpStatus?: number;
  response?: ApiEnvelope;
}
