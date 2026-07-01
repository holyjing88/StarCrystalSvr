/**
 * 依次跑服务端 go test 回归 + Unity 客户端回归（需 npm run dev）。
 */
const base = (process.env.VITE_URL ?? 'http://localhost:5174').replace(/\/$/, '');

async function runServerRegression() {
  console.log('\n=== 服务端回归 ===');
  const t0 = Date.now();
  const res = await fetch(`${base}/dev/regression/server`, { method: 'POST' });
  const data = await res.json();
  if (!res.ok) throw new Error(`server regression HTTP ${res.status}: ${JSON.stringify(data).slice(0, 300)}`);
  const results = data.results ?? [];
  let pass = 0;
  let fail = 0;
  for (const r of results) {
    if (r.passed) pass++;
    else {
      fail++;
      console.log('FAIL', r.id, (r.error ?? '').slice(0, 120));
    }
  }
  console.log(`服务端: ${pass}/${results.length} PASS, ${fail} FAIL (${Math.round((Date.now() - t0) / 1000)}s)`);
  if (data.error) console.log('error:', data.error);
  return fail === 0;
}

async function runClientRegression() {
  console.log('\n=== 客户端回归 ===');
  const pollMs = 5000;
  const maxMs = Number(process.env.CLIENT_POLL_MAX_MS ?? 50 * 60 * 1000);
  const url = `${base}/dev/regression/client?force=1`;
  const p = await fetch(url, { method: 'POST' });
  const startBody = await p.json();
  console.log('POST', p.status, JSON.stringify(startBody).slice(0, 180));
  const t0 = Date.now();
  while (true) {
    const st = await (await fetch(`${base}/dev/regression/client/status`)).json();
    const prog = st.progress;
    const line = `[${Math.round((Date.now() - t0) / 1000)}s] ${st.status} phase=${st.phase} ${prog ? `${prog.completed}/${prog.total}` : ''}`;
    if (!globalThis.__cl || globalThis.__cl !== line) {
      console.log(line);
      globalThis.__cl = line;
    }
    if (st.status === 'done' || st.status === 'error') break;
    if (Date.now() - t0 > maxMs) throw new Error('client regression timeout');
    await new Promise((r) => setTimeout(r, pollMs));
  }
  const st = await (await fetch(`${base}/dev/regression/client/status`)).json();
  const results = st.data?.results ?? [];
  let pass = 0;
  let fail = 0;
  for (const r of results) {
    if (r.passed) pass++;
    else {
      fail++;
      console.log('FAIL', r.id, (r.error ?? '').slice(0, 120));
    }
  }
  const total = results.length || 31;
  console.log(`客户端: ${pass}/${total} PASS, ${fail} FAIL (${Math.round((Date.now() - t0) / 1000)}s)`);
  return fail === 0;
}

async function main() {
  const serverOk = await runServerRegression();
  const clientOk = await runClientRegression();
  console.log('\n=== 汇总 ===');
  console.log(`服务端: ${serverOk ? 'PASS' : 'FAIL'}`);
  console.log(`客户端: ${clientOk ? 'PASS' : 'FAIL'}`);
  process.exit(serverOk && clientOk ? 0 : 1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
