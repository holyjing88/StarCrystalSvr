/**
 * POST /dev/regression/client 并轮询至结束，打印 PASS/FAIL 汇总。
 * 需先 npm run dev。
 */
const base = (process.env.VITE_URL ?? 'http://localhost:5174').replace(/\/$/, '');
const pollMs = 5000;
const maxMs = Number(process.env.CLIENT_POLL_MAX_MS ?? 50 * 60 * 1000);

async function getStatus() {
  const r = await fetch(`${base}/dev/regression/client/status`);
  return r.json();
}

async function main() {
  let st = await getStatus();
  if (!st.running) {
    const url =
      st.status === 'done' || st.status === 'failed'
        ? `${base}/dev/regression/client?force=1`
        : `${base}/dev/regression/client`;
    const p = await fetch(url, { method: 'POST' });
    const body = await p.json();
    console.log('POST', p.status, JSON.stringify(body).slice(0, 200));
    await new Promise((r) => setTimeout(r, 2000));
  }
  const t0 = Date.now();
  while (true) {
    st = await getStatus();
    const prog = st.progress;
    const line = `[${Math.round((Date.now() - t0) / 1000)}s] status=${st.status} phase=${st.phase} ${prog ? `${prog.completed}/${prog.total}` : ''}`;
    if (!globalThis.__lastLine || globalThis.__lastLine !== line) {
      console.log(line);
      globalThis.__lastLine = line;
    }
    if (st.status === 'done' || st.status === 'error') break;
    if (!st.running && st.status !== 'running') break;
    if (Date.now() - t0 > maxMs) throw new Error('timeout');
    await new Promise((r) => setTimeout(r, pollMs));
  }
  const results = st.data?.results ?? [];
  const cases = st.progress?.cases ?? results;
  let pass = 0;
  let fail = 0;
  for (const c of results.length ? results : cases) {
    const ok = c.passed === true || c.result === 'PASS' || c.status === 'pass';
    if (ok) pass++;
    else if (c.passed === false || c.status === 'fail' || c.status === 'FAIL') {
      fail++;
      console.log('FAIL', c.id, (c.error ?? c.message ?? '').slice(0, 160));
    }
  }
  const total = results.length || cases.length || 31;
  console.log(`\n=== ${pass}/${total} PASS, ${fail} FAIL ===`);
  console.log(st.summary ?? st.log?.slice(-400) ?? '');
  process.exit(fail > 0 ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
