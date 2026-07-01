/**
 * 部署验收：Linux nginx 上的 idip-webclient 静态站点可访问。
 * 设置 IDIP_WEBCLIENT_URL=https://192.168.75.99 后运行 npm run regression。
 */
import { describe, expect, it } from 'vitest';

const webUrl = (process.env.IDIP_WEBCLIENT_URL ?? '').replace(/\/$/, '');

// Linux 测试机使用自签证书
if (webUrl.startsWith('https://')) {
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';
}

describe('idip-webclient static deploy', () => {
  it('IDIP-WEB-001 index.html served', async (ctx) => {
    if (!webUrl) ctx.skip('IDIP_WEBCLIENT_URL not set');
    const res = await fetch(`${webUrl}/`, { redirect: 'follow' });
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toMatch(/StarCrystal|IDIP|运营/i);
  });

  it('IDIP-WEB-002 same-origin /idip proxy reachable', async (ctx) => {
    if (!webUrl) ctx.skip('IDIP_WEBCLIENT_URL not set');
    const res = await fetch(`${webUrl}/idip/v1/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'bad', password: 'bad' }),
    });
    expect(res.status).toBe(401);
    const body = (await res.json()) as { code?: number };
    expect(body.code).toBe(1401);
  });

  it('IDIP-WEB-003 vite bundle under /assets/ served (not proxied to API 404)', async (ctx) => {
    if (!webUrl) ctx.skip('IDIP_WEBCLIENT_URL not set');
    const indexRes = await fetch(`${webUrl}/`, { redirect: 'follow' });
    const html = await indexRes.text();
    const jsMatch = html.match(/src="(\/assets\/[^"]+\.js)"/);
    expect(jsMatch, 'index.html should reference /assets/*.js').toBeTruthy();
    const jsUrl = `${webUrl}${jsMatch![1]}`;
    const jsRes = await fetch(jsUrl, { redirect: 'follow' });
    expect(jsRes.status).toBe(200);
    const ct = jsRes.headers.get('content-type') ?? '';
    expect(ct).toMatch(/javascript|text\/plain/i);
    const body = await jsRes.text();
    expect(body.length).toBeGreaterThan(1000);
  });
});
