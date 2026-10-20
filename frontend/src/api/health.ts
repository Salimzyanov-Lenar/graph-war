import type { Fetch } from './rooms';

export type ServerStatus = 'checking' | 'ready' | 'not_ready' | 'down';

/** `GET /readyz` (готов, БД доступна) и `GET /healthz` (процесс жив). Ошибка сети — сервер недоступен. */
export async function checkServerStatus(fetchImpl: Fetch = (...args) => fetch(...args)): Promise<ServerStatus> {
  try {
    if ((await fetchImpl('/readyz')).ok) return 'ready';
    return (await fetchImpl('/healthz')).ok ? 'not_ready' : 'down';
  } catch {
    return 'down';
  }
}

export const SERVER_STATUS_TEXT: Record<ServerStatus, string> = {
  checking: 'Сервер: проверяем…',
  ready: 'Сервер: готов',
  not_ready: 'Сервер запущен, но не готов (нет базы данных).',
  down: 'Сервер недоступен. Он запущен?',
};
