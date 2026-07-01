import { execFile, spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
import type { Plugin } from 'vite';
import { CLIENT_REGRESSION_CATALOG } from './src/regression/clientCatalog';
import {
  buildClientProgressFromUtp,
  formatClientCaseProgressLine,
  parseUtpTestEndsFromLog,
  type ClientRegressionProgress,
} from './src/regression/parseUnityUtpLog';
import { uniqueGoTests } from './src/regression/catalogUtils';
import { SERVER_REGRESSION_CATALOG } from './src/regression/serverCatalog';

export interface RegressionRunnerOptions {
  /** 仓库根（含 server-go、StarCrystal2022） */
  repoRoot?: string;
}

interface GoTestEvent {
  Action?: string;
  Test?: string;
  Package?: string;
  Elapsed?: number;
  Output?: string;
}

function repoRootFromPlugin(): string {
  return path.resolve(import.meta.dirname, '..');
}

function runProcess(
  cmd: string,
  args: string[],
  cwd: string,
  timeoutMs: number,
  envExtra?: Record<string, string>,
): Promise<{ stdout: string; stderr: string; code: number | null }> {
  return new Promise((resolve) => {
    // 勿用 shell:true：Windows 下 -run 'A|B' 会被 cmd 当成管道，go test 完全跑不起来
    const child = spawn(cmd, args, {
      cwd,
      shell: false,
      windowsHide: true,
      env: { ...process.env, ...envExtra },
    });
    let stdout = '';
    let stderr = '';
    child.stdout?.on('data', (d) => {
      stdout += String(d);
    });
    child.stderr?.on('data', (d) => {
      stderr += String(d);
    });
    const timer = setTimeout(() => {
      child.kill('SIGTERM');
    }, timeoutMs);
    child.on('close', (code) => {
      clearTimeout(timer);
      resolve({ stdout, stderr, code });
    });
    child.on('error', (e) => {
      clearTimeout(timer);
      resolve({ stdout, stderr: stderr + e.message, code: 1 });
    });
  });
}

function parseGoTestJson(stdout: string): Map<string, { passed: boolean; skipped: boolean; ms: number; error: string }> {
  const out = new Map<string, { passed: boolean; skipped: boolean; ms: number; error: string }>();
  const failBuf = new Map<string, string>();
  for (const line of stdout.split(/\r?\n/)) {
    if (!line.trim()) continue;
    let ev: GoTestEvent;
    try {
      ev = JSON.parse(line) as GoTestEvent;
    } catch {
      continue;
    }
    const name = ev.Test;
    if (!name) continue;
    if (ev.Action === 'output' && ev.Output) {
      failBuf.set(name, (failBuf.get(name) ?? '') + ev.Output);
    }
    if (ev.Action === 'pass') {
      out.set(name, { passed: true, skipped: false, ms: Math.round((ev.Elapsed ?? 0) * 1000), error: '' });
    }
    if (ev.Action === 'skip') {
      out.set(name, { passed: false, skipped: true, ms: 0, error: 'skipped' });
    }
    if (ev.Action === 'fail') {
      out.set(name, {
        passed: false,
        skipped: false,
        ms: Math.round((ev.Elapsed ?? 0) * 1000),
        error: (failBuf.get(name) ?? 'fail').trim().slice(0, 500),
      });
    }
  }
  return out;
}

function parseNUnitXml(xml: string): Map<string, { passed: boolean; skipped: boolean; ms: number; error: string }> {
  const out = new Map<string, { passed: boolean; skipped: boolean; ms: number; error: string }>();
  const caseRe =
    /<test-case\b([^>]*)\/?>/gi;
  let m: RegExpExecArray | null;
  while ((m = caseRe.exec(xml)) !== null) {
    const attrs = m[1] ?? '';
    const read = (key: string): string | undefined => {
      const re = new RegExp(`\\b${key}="([^"]*)"`);
      return re.exec(attrs)?.[1];
    };
    const fullName = read('fullname') ?? read('name');
    if (!fullName) continue;
    const short = fullName.split('.').pop() ?? fullName;
    const result = read('result') ?? '';
    const ms = Math.round(parseFloat(read('duration') ?? read('time') ?? '0') * 1000);
    out.set(short, {
      passed: result === 'Passed',
      skipped: result === 'Skipped' || result === 'Inconclusive',
      ms,
      error: result === 'Failed' ? 'Failed (see Unity log)' : result === 'Skipped' ? 'Ignored/Skipped' : '',
    });
  }
  return out;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function filterAlivePids(pids: number[]): number[] {
  return pids.filter((pid) => {
    try {
      process.kill(pid, 0);
      return true;
    } catch (e) {
      return (e as NodeJS.ErrnoException).code === 'EPERM';
    }
  });
}

function unityLockfilePath(projectAbs: string): string {
  return path.join(path.resolve(projectAbs), 'Temp', 'UnityLockfile');
}

function tryRemoveUnityLockfile(projectAbs: string): boolean {
  const lock = unityLockfilePath(projectAbs);
  try {
    if (fs.existsSync(lock)) fs.unlinkSync(lock);
    return true;
  } catch {
    return false;
  }
}

/** 列出仍占用该工程的 Unity.exe PID（含 CommandLine 为空的 Editor） */
async function listUnityPidsForProject(projectAbs: string): Promise<number[]> {
  const resolved = path.resolve(projectAbs);
  const forward = resolved.replace(/\\/g, '/');
  const folder = path.basename(resolved);

  if (process.platform !== 'win32') {
    return [];
  }

  const projEsc = resolved.replace(/'/g, "''");
  const fwdEsc = forward.replace(/'/g, "''");
  const folderEsc = folder.replace(/'/g, "''");
  const ps = [
    `$proj = '${projEsc}'`,
    `$fwd = '${fwdEsc}'`,
    `$folder = '${folderEsc}'`,
    '$pids = @()',
    "Get-CimInstance Win32_Process -Filter \"Name='Unity.exe'\" | ForEach-Object {",
    '  $cl = $_.CommandLine',
    '  if (-not $cl) { return }',
    '  if ($cl -like "*$proj*" -or $cl -like "*$fwd*" -or $cl -like "*$folder*") { $pids += $_.ProcessId }',
    '}',
    "($pids | Select-Object -Unique) -join ','",
  ].join('\n');

  try {
    const { stdout } = await execFileAsync(
      'powershell.exe',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', ps],
      { windowsHide: true, timeout: 30_000 },
    );
    return [
      ...new Set(
        stdout
          .trim()
          .split(',')
          .map((x) => Number(x.trim()))
          .filter((n) => Number.isFinite(n) && n > 0),
      ),
    ];
  } catch {
    return [];
  }
}

/** 回归前结束占用该工程的 Unity.exe（含 CommandLine 为空的 Editor、锁文件存在时的残留） */
async function forceCloseUnityForProject(projectAbs: string): Promise<{
  killed: number;
  pids: number[];
  log: string;
}> {
  if (process.env.SC_FORCE_CLOSE_UNITY === '0') {
    return { killed: 0, pids: [], log: '已跳过强制关闭 Unity（SC_FORCE_CLOSE_UNITY=0）' };
  }

  const resolved = path.resolve(projectAbs);
  const forward = resolved.replace(/\\/g, '/');

  if (process.platform === 'win32') {
    const lock = unityLockfilePath(resolved);
    if (fs.existsSync(lock)) {
      try {
        await execFileAsync(
          'powershell.exe',
          [
            '-NoProfile',
            '-ExecutionPolicy',
            'Bypass',
            '-Command',
            "Get-Process Unity -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue",
          ],
          { windowsHide: true, timeout: 20_000 },
        );
      } catch {
        /* ignore */
      }
      await sleep(1500);
    }

    const allKilled: number[] = [];
    for (let round = 0; round < 3; round++) {
      const pids = filterAlivePids(await listUnityPidsForProject(resolved));
      if (pids.length === 0) break;
      const pidList = pids.join(',');
      try {
        await execFileAsync(
          'powershell.exe',
          [
            '-NoProfile',
            '-ExecutionPolicy',
            'Bypass',
            '-Command',
            `$ids = @(${pidList}); foreach ($id in $ids) { Stop-Process -Id $id -Force -ErrorAction SilentlyContinue }`,
          ],
          { windowsHide: true, timeout: 20_000 },
        );
        allKilled.push(...pids);
      } catch {
        for (const pid of pids) {
          try {
            process.kill(pid, 'SIGKILL');
            allKilled.push(pid);
          } catch {
            /* already gone */
          }
        }
      }
      await sleep(1500);
    }
    tryRemoveUnityLockfile(resolved);
    if (allKilled.length > 0) await sleep(2000);
    return {
      killed: allKilled.length,
      pids: [...new Set(allKilled)],
      log:
        allKilled.length > 0
          ? `已强制结束 Unity 进程: PID ${[...new Set(allKilled)].join(', ')}`
          : '未发现需结束的 Unity 进程',
    };
  }

  try {
    await execFileAsync('pkill', ['-f', forward], { timeout: 15_000 });
    await sleep(2000);
    return { killed: -1, pids: [], log: '已执行 pkill -f（非 Windows）' };
  } catch {
    return { killed: 0, pids: [], log: '非 Windows 且 pkill 未结束进程（可手动关闭 Editor）' };
  }
}

async function waitForUnityProjectFree(projectAbs: string, maxMs = 20_000): Promise<string> {
  const resolved = path.resolve(projectAbs);
  const deadline = Date.now() + maxMs;
  while (Date.now() < deadline) {
    const pids = filterAlivePids(await listUnityPidsForProject(resolved));
    const lock = unityLockfilePath(resolved);
    if (pids.length === 0) {
      if (!fs.existsSync(lock)) return '工程已释放';
      tryRemoveUnityLockfile(resolved);
      if (!fs.existsSync(lock)) return '工程已释放（已清理锁文件）';
    }
    await sleep(500);
  }
  const remain = filterAlivePids(await listUnityPidsForProject(resolved));
  return remain.length > 0
    ? `等待工程释放超时，仍有 Unity PID: ${remain.join(', ')}`
    : '等待 UnityLockfile 释放超时';
}

function detectUnityLogFailure(log: string): string | undefined {
  if (
    /another Unity instance is running/i.test(log) ||
    /ProjectAlreadyOpenInAnotherInstance/i.test(log) ||
    /already open in another instance/i.test(log)
  ) {
    return '关闭 Unity 后工程仍被占用；请手动结束所有 Unity.exe 或删除 Temp/UnityLockfile 后重试';
  }
  if (/Scripts have compiler errors/i.test(log)) {
    return 'Unity 脚本编译失败，请先在 Editor 中修复编译错误（见 tmp-regression/unity_*.log）';
  }
  if (/is not a valid directory name/i.test(log)) {
    return 'Unity 日志/结果路径无效，请更新 idip-webclient 后重启 npm run dev';
  }
  return undefined;
}

function tailFile(filePath: string, maxChars = 6000): string {
  try {
    if (!fs.existsSync(filePath)) return '';
    const buf = fs.readFileSync(filePath);
    const text = buf.toString('utf8');
    return text.length <= maxChars ? text : text.slice(-maxChars);
  } catch {
    return '';
  }
}

/** Unity 官方 CLI 跑测试（与 StarCrystal2022/doc/客户端测试用例.md §1.3 一致） */
function unityRunTestsArgs(
  testPlatform: 'EditMode' | 'PlayMode',
  project: string,
  resultsXml: string,
  logFile: string,
): string[] {
  const assembly =
    testPlatform === 'EditMode'
      ? 'StarCrystal.Client.Tests.EditMode'
      : 'StarCrystal.Client.Tests.PlayMode';
  return [
    '-batchmode',
    '-nographics',
    '-automated',
    '-projectPath',
    project,
    '-runTests',
    '-testPlatform',
    testPlatform.toLowerCase(),
    '-assemblyNames',
    assembly,
    '-testResults',
    resultsXml,
    '-logFile',
    logFile,
    // 勿加 -quit：会在跑测试前退出（见 smoke_edit.log vs smoke_edit2.xml）
  ];
}

function defaultUnityPath(): string | null {
  const env = process.env.UNITY_EDITOR ?? process.env.UNITY_PATH;
  if (env && fs.existsSync(env)) return env;
  const hub = 'C:\\Program Files\\Unity\\Hub\\Editor';
  if (!fs.existsSync(hub)) return null;
  const versions = fs.readdirSync(hub).filter((v) => fs.existsSync(path.join(hub, v, 'Editor', 'Unity.exe')));
  versions.sort().reverse();
  if (versions.length === 0) return null;
  return path.join(hub, versions[0]!, 'Editor', 'Unity.exe');
}

async function runServerGoTests(serverGoDir: string): Promise<{
  results: { id: string; passed: boolean; durationMs?: number; error?: string; skipped?: boolean }[];
  log: string;
  error?: string;
}> {
  const runArg = uniqueGoTests(SERVER_REGRESSION_CATALOG).join('|');
  const { stdout, stderr, code } = await runProcess(
    'go',
    ['test', './internal/api/', './internal/service/', '-count=1', '-json', '-timeout', '8m', '-run', runArg],
    serverGoDir,
    8 * 60 * 1000,
  );
  const log = (stdout + '\n' + stderr).slice(-12000);
  const spawnFailed = code === 1 && stdout === '' && stderr.includes('ENOENT');
  const parsed = parseGoTestJson(stdout);
  const spawnHint = spawnFailed
    ? '未找到 go 命令，请安装 Go 并加入 PATH'
    : code !== 0 && code !== null && parsed.size === 0
      ? (stderr || stdout).trim().slice(0, 200) || `go test exit ${code}`
      : undefined;
  const results = SERVER_REGRESSION_CATALOG.map(({ id, goTest }) => {
    const r = goTest ? parsed.get(goTest) : undefined;
    if (!r) {
      return {
        id,
        passed: false,
        error: spawnHint ?? (code === 0 ? `未在 go test 输出中找到 ${goTest}` : `go test exit ${code}`),
      };
    }
    return {
      id,
      passed: r.passed,
      skipped: r.skipped,
      durationMs: r.ms,
      error: r.error || undefined,
    };
  });
  return {
    results,
    log,
    error: code !== 0 && results.every((x) => !x.passed) ? `go test 退出码 ${code}` : undefined,
  };
}

type ClientRegressionResult = {
  results: { id: string; passed: boolean; durationMs?: number; error?: string; skipped?: boolean }[];
  log: string;
  error?: string;
};

type ClientRegressionHooks = {
  onPhase?: (phase: string) => void;
  appendLog?: (chunk: string) => void;
};

async function preflightServerForClientRegression(): Promise<string | undefined> {
  const base = (process.env.STARCHRYSTAL_API_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
  try {
    const health = await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(5000) });
    if (!health.ok) return `server-go /healthz 异常: HTTP ${health.status}`;
    const smsRes = await fetch(`${base}/api/v1/auth/sendverificationcode`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        purpose: 'register',
        account: `+86138${String(Date.now() % 100000000).padStart(8, '0')}`,
        channel: 'phone',
      }),
      signal: AbortSignal.timeout(8000),
    });
    const sms = (await smsRes.json()) as { code?: number; data?: { devVerifyCode?: string }; message?: string };
    if (sms.code === 0 && sms.data?.devVerifyCode) return undefined;
    return (
      'server-go 未返回 devVerifyCode（PlayMode 注册/邀请用例会失败）。' +
      '请执行 server-go/release/start.ps1 重启并确认 AUTH_SMS_MOCK=1'
    );
  } catch (e) {
    return `无法连接 server-go（${base}）: ${e instanceof Error ? e.message : String(e)}`;
  }
}

async function runUnityClientTests(
  repoRoot: string,
  hooks: ClientRegressionHooks = {},
): Promise<ClientRegressionResult> {
  const { onPhase, appendLog } = hooks;
  const note = (s: string) => {
    appendLog?.(s.endsWith('\n') ? s : `${s}\n`);
  };
  const unity = defaultUnityPath();
  const project = path.join(repoRoot, 'StarCrystal2022');
  if (!unity) {
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: '未找到 Unity.exe，请设置环境变量 UNITY_EDITOR',
      })),
      log: '',
      error: 'UNITY_EDITOR 未配置',
    };
  }
  if (!fs.existsSync(project)) {
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: `工程不存在: ${project}`,
      })),
      log: '',
      error: 'StarCrystal2022 路径不存在',
    };
  }

  const tmpDir = path.join(import.meta.dirname, 'tmp-regression');
  fs.mkdirSync(tmpDir, { recursive: true });
  const editXml = path.resolve(tmpDir, 'client_editmode.xml');
  const playXml = path.resolve(tmpDir, 'client_playmode.xml');
  const editLog = path.resolve(tmpDir, 'unity_edit.log');
  const playLog = path.resolve(tmpDir, 'unity_play.log');

  onPhase?.('closing');
  const closed = await forceCloseUnityForProject(project);
  let log = `[regression] ${closed.log}\n`;
  note(log);

  onPhase?.('waiting_unlock');
  const waitMsg = await waitForUnityProjectFree(project);
  log += `[regression] ${waitMsg}\n`;
  note(`[regression] ${waitMsg}\n`);
  if (waitMsg.includes('超时')) {
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: waitMsg,
      })),
      log,
      error: waitMsg,
    };
  }

  onPhase?.('preflight');
  const preflightErr = await preflightServerForClientRegression();
  if (preflightErr) {
    log += `[regression] ${preflightErr}\n`;
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: preflightErr,
      })),
      log,
      error: preflightErr,
    };
  }
  log += '[regression] server-go 预检通过（devVerifyCode 可用）\n';
  note(log);

  onPhase?.('editmode');
  const edit = await runProcess(
    unity,
    unityRunTestsArgs('EditMode', project, editXml, editLog),
    repoRoot,
    20 * 60 * 1000,
  );
  log += edit.stdout + edit.stderr;
  if (fs.existsSync(editLog)) log += '\n' + tailFile(editLog, 4000);
  if (edit.code === null) log += '\n[regression] EditMode Unity 超时或被终止\n';
  note(log.slice(-2000));

  onPhase?.('playmode');
  const play = await runProcess(
    unity,
    unityRunTestsArgs('PlayMode', project, playXml, playLog),
    repoRoot,
    40 * 60 * 1000,
  );
  log += play.stdout + play.stderr;
  if (fs.existsSync(playLog)) log += '\n' + tailFile(playLog, 4000);
  if (play.code === null) log += '\n[regression] PlayMode Unity 超时或被终止\n';
  note(log.slice(-2000));

  onPhase?.('parsing');
  if (!fs.existsSync(editXml) && !fs.existsSync(playXml)) {
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: 'Unity 未生成测试结果 XML，请查看 tmp-regression/unity_*.log',
      })),
      log,
      error: '未生成 client_editmode.xml / client_playmode.xml',
    };
  }
  const fatal = detectUnityLogFailure(log);
  if (fatal) {
    return {
      results: CLIENT_REGRESSION_CATALOG.map(({ id }) => ({
        id,
        passed: false,
        error: fatal,
      })),
      log: log.slice(-12000),
      error: fatal,
    };
  }

  const merged = new Map<string, { passed: boolean; skipped: boolean; ms: number; error: string }>();
  for (const f of [editXml, playXml]) {
    if (fs.existsSync(f)) {
      const parsed = parseNUnitXml(fs.readFileSync(f, 'utf8'));
      for (const [k, v] of parsed) merged.set(k, v);
    }
  }

  const results = CLIENT_REGRESSION_CATALOG.map(({ id, unityTest }) => {
    const r = unityTest ? merged.get(unityTest) : undefined;
    if (!r) {
      return {
        id,
        passed: false,
        error: `未在 Unity 结果 XML 中找到 ${unityTest}（可能 Ignore 或未纳入 batch）`,
      };
    }
    return {
      id,
      passed: r.passed,
      skipped: r.skipped,
      durationMs: r.ms,
      error: r.error || undefined,
    };
  });

  return {
    results,
    log: log.slice(-12000),
    error:
      edit.code !== 0 || play.code !== 0
        ? `Unity 退出码 edit=${edit.code} play=${play.code}`
        : undefined,
  };
}

type ClientJobSnapshot =
  | { status: 'idle' }
  | {
      status: 'running';
      phase: string;
      log: string;
      startedAt: number;
      unityLogFile?: string;
      progress?: ClientRegressionProgress;
    }
  | { status: 'done'; phase: string; log: string; data: ClientRegressionResult; progress?: ClientRegressionProgress }
  | { status: 'error'; phase: string; log: string; error: string; data?: ClientRegressionResult; progress?: ClientRegressionProgress };

export function regressionRunnerPlugin(options: RegressionRunnerOptions = {}): Plugin {
  const repoRoot = options.repoRoot ?? repoRootFromPlugin();
  const serverGoDir = path.join(repoRoot, 'server-go');
  let clientJob: ClientJobSnapshot = { status: 'idle' };
  let clientUnityLogFiles: { edit: string; play: string } | null = null;
  const clientLogReadOffsets = { edit: 0, play: 0 };
  const clientUtpEnds = new Map<string, ReturnType<typeof parseUtpTestEndsFromLog>[number]>();
  const clientLoggedCaseIds = new Set<string>();

  function resetClientProgressTracking(): void {
    clientLogReadOffsets.edit = 0;
    clientLogReadOffsets.play = 0;
    clientUtpEnds.clear();
    clientLoggedCaseIds.clear();
  }

  function drainUtpFromUnityLog(logPath: string, key: 'edit' | 'play'): void {
    if (!fs.existsSync(logPath)) return;
    const stat = fs.statSync(logPath);
    if (stat.size <= clientLogReadOffsets[key]) return;
    const fd = fs.openSync(logPath, 'r');
    try {
      const len = stat.size - clientLogReadOffsets[key];
      const buf = Buffer.alloc(len);
      fs.readSync(fd, buf, 0, len, clientLogReadOffsets[key]);
      clientLogReadOffsets[key] = stat.size;
      for (const e of parseUtpTestEndsFromLog(buf.toString('utf8'))) {
        clientUtpEnds.set(e.fullName, e);
      }
    } finally {
      fs.closeSync(fd);
    }
  }

  function readClientRegressionProgress(): ClientRegressionProgress {
    if (clientUnityLogFiles) {
      drainUtpFromUnityLog(clientUnityLogFiles.edit, 'edit');
      drainUtpFromUnityLog(clientUnityLogFiles.play, 'play');
    }
    return buildClientProgressFromUtp(CLIENT_REGRESSION_CATALOG, [...clientUtpEnds.values()]);
  }

  function logNewClientCaseProgress(progress: ClientRegressionProgress): void {
    for (const c of progress.cases) {
      if (c.status === 'pending' || clientLoggedCaseIds.has(c.id)) continue;
      clientLoggedCaseIds.add(c.id);
      console.log(formatClientCaseProgressLine(clientLoggedCaseIds.size, progress.total, c));
    }
  }

  function json(res: { statusCode: number; setHeader: (k: string, v: string) => void; end: (s: string) => void }, code: number, body: unknown) {
    res.statusCode = code;
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(body));
  }

  function enrichClientJobSnapshot(job: ClientJobSnapshot): ClientJobSnapshot {
    if (job.status !== 'running' || !clientUnityLogFiles) return job;
    const logFile =
      job.phase === 'playmode' ? clientUnityLogFiles.play : clientUnityLogFiles.edit;
    const progress = readClientRegressionProgress();
    logNewClientCaseProgress(progress);
    const live = tailFile(logFile, 5000);
    return {
      ...job,
      unityLogFile: logFile,
      progress,
      log: live ? `${job.log}\n--- Unity 日志尾部 ---\n${live}`.slice(-24_000) : job.log,
    };
  }

  async function startClientRegressionJob() {
    const tmpDir = path.join(import.meta.dirname, 'tmp-regression');
    fs.mkdirSync(tmpDir, { recursive: true });
    clientUnityLogFiles = {
      edit: path.resolve(tmpDir, 'unity_edit.log'),
      play: path.resolve(tmpDir, 'unity_play.log'),
    };
    resetClientProgressTracking();
    clientJob = { status: 'running', phase: 'starting', log: '', startedAt: Date.now() };
    try {
      const data = await runUnityClientTests(repoRoot, {
        onPhase: (phase) => {
          if (clientJob.status === 'running') {
            const logFile =
              phase === 'playmode'
                ? clientUnityLogFiles?.play
                : phase === 'editmode'
                  ? clientUnityLogFiles?.edit
                  : undefined;
            clientJob = { ...clientJob, phase, unityLogFile: logFile };
          }
        },
        appendLog: (chunk) => {
          if (clientJob.status === 'running') {
            clientJob = { ...clientJob, log: (clientJob.log + chunk).slice(-24_000) };
          }
        },
      });
      const progress = readClientRegressionProgress();
      logNewClientCaseProgress(progress);
      const phase = data.error ? 'failed' : 'done';
      clientJob = { status: 'done', phase, log: data.log, data, progress };
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      clientJob = {
        status: 'error',
        phase: 'error',
        log: clientJob.status === 'running' ? clientJob.log : '',
        error: msg,
      };
    }
  }

  return {
    name: 'starcrystal-regression-runner',
    configureServer(server) {
      server.middlewares.use('/dev/regression/server', async (req, res) => {
        if (req.method !== 'POST') {
          res.statusCode = 405;
          res.end('Method Not Allowed');
          return;
        }
        try {
          const data = await runServerGoTests(serverGoDir);
          res.setHeader('Content-Type', 'application/json');
          res.statusCode = 200;
          res.end(JSON.stringify(data));
        } catch (e) {
          res.statusCode = 500;
          res.end(JSON.stringify({ error: e instanceof Error ? e.message : String(e), results: [] }));
        }
      });

      server.middlewares.use('/dev/regression/client/status', (req, res) => {
        if (req.method !== 'GET') {
          res.statusCode = 405;
          res.end('Method Not Allowed');
          return;
        }
        json(res, 200, enrichClientJobSnapshot(clientJob));
      });

      server.middlewares.use('/dev/regression/client', async (req, res) => {
        if (req.method === 'GET') {
          json(res, 200, enrichClientJobSnapshot(clientJob));
          return;
        }
        if (req.method !== 'POST') {
          res.statusCode = 405;
          res.end('Method Not Allowed');
          return;
        }
        const force = typeof req.url === 'string' && req.url.includes('force=1');
        if (clientJob.status === 'running' && !force) {
          json(res, 200, {
            started: true,
            joined: true,
            job: clientJob,
          });
          return;
        }
        if (clientJob.status === 'running' && force) {
          await forceCloseUnityForProject(path.join(repoRoot, 'StarCrystal2022'));
          clientJob = { status: 'idle' };
        }
        void startClientRegressionJob();
        json(res, 202, { started: true, job: clientJob });
      });
    },
  };
}
