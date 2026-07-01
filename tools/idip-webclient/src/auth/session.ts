const SESSION_KEY = 'idip.sessionToken';
const USERNAME_KEY = 'idip.username';
const HEARTBEAT_INTERVAL_KEY = 'idip.heartbeatIntervalSec';
const DEFAULT_HEARTBEAT_INTERVAL_SEC = 30;

let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
let onSessionExpired: (() => void) | null = null;
let visibilityHandler: (() => void) | null = null;

export function getSessionToken(): string {
  return sessionStorage.getItem(SESSION_KEY) ?? '';
}

export function getSessionUsername(): string {
  return sessionStorage.getItem(USERNAME_KEY) ?? '';
}

export function setSession(token: string, username: string): void {
  sessionStorage.setItem(SESSION_KEY, token);
  sessionStorage.setItem(USERNAME_KEY, username);
}

export function setHeartbeatIntervalSec(sec: number): void {
  if (sec > 0) {
    sessionStorage.setItem(HEARTBEAT_INTERVAL_KEY, String(sec));
  }
}

/** 客户端上报间隔：服务端建议值的 60%，最低 5s，避免与 idle 超时撞车。 */
export function getHeartbeatIntervalMs(): number {
  const raw = sessionStorage.getItem(HEARTBEAT_INTERVAL_KEY);
  const sec = raw ? Number(raw) : DEFAULT_HEARTBEAT_INTERVAL_SEC;
  const safeSec = Number.isFinite(sec) && sec > 0 ? sec : DEFAULT_HEARTBEAT_INTERVAL_SEC;
  return Math.max(5000, Math.floor(safeSec * 1000 * 0.6));
}

export function clearSession(): void {
  sessionStorage.removeItem(SESSION_KEY);
  sessionStorage.removeItem(USERNAME_KEY);
  stopHeartbeat();
}

export function setOnSessionExpired(cb: () => void): void {
  onSessionExpired = cb;
}

export function stopHeartbeat(): void {
  if (heartbeatTimer != null) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
  if (visibilityHandler != null) {
    document.removeEventListener('visibilitychange', visibilityHandler);
    visibilityHandler = null;
  }
}

/** 定期心跳 + 切回标签页立即补发；持续上报则保持登录。 */
export function startHeartbeat(beat: () => Promise<boolean>): void {
  stopHeartbeat();
  const intervalMs = getHeartbeatIntervalMs();
  const runBeat = () => {
    void beat().then((ok) => {
      if (!ok) {
        clearSession();
        onSessionExpired?.();
      }
    });
  };
  runBeat();
  heartbeatTimer = setInterval(runBeat, intervalMs);
  visibilityHandler = () => {
    if (document.visibilityState === 'visible') {
      runBeat();
    }
  };
  document.addEventListener('visibilitychange', visibilityHandler);
}
