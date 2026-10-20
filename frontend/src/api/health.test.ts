import { describe, expect, it } from 'vitest';
import { checkServerStatus } from './health';

const reply = (status: number) => new Response(null, { status });
const withRoutes = (routes: Record<string, number>) =>
  (async (url: RequestInfo | URL) => reply(routes[String(url)] ?? 404)) as unknown as typeof fetch;

describe('checkServerStatus', () => {
  it('ready, если /readyz отвечает 200', async () => {
    await expect(checkServerStatus(withRoutes({ '/readyz': 200, '/healthz': 200 }))).resolves.toBe('ready');
  });

  it('not_ready, если жив только /healthz', async () => {
    await expect(checkServerStatus(withRoutes({ '/readyz': 503, '/healthz': 200 }))).resolves.toBe('not_ready');
  });

  it('down, если нет ни того ни другого или сеть недоступна', async () => {
    await expect(checkServerStatus(withRoutes({ '/readyz': 502, '/healthz': 502 }))).resolves.toBe('down');
    await expect(checkServerStatus((async () => Promise.reject(new TypeError('Failed to fetch'))) as unknown as typeof fetch)).resolves.toBe('down');
  });
});
