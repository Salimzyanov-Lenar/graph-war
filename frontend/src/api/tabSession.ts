const KEY = 'graphwar.tabToken';
const WS_PROTOCOL = 'graphwar.v1';

/** Личность вкладки: токен лежит в sessionStorage, поэтому две вкладки одного браузера — два разных игрока. */
export interface TabSession {
  get(): string | undefined;
  set(token: string): void;
}

export function memoryTabSession(initial?: string): TabSession {
  let token = initial;
  return { get: () => token, set: (next) => void (token = next) };
}

export function browserTabSession(): TabSession {
  const fallback = memoryTabSession();
  return {
    get() {
      try {
        return sessionStorage.getItem(KEY) ?? fallback.get();
      } catch {
        return fallback.get();
      }
    },
    set(token) {
      fallback.set(token);
      try {
        sessionStorage.setItem(KEY, token);
      } catch {
        // без sessionStorage личность живёт до перезагрузки вкладки
      }
    },
  };
}

/** Браузерный WebSocket не умеет заголовки: токен едет вторым subprotocol'ом, сервер выбирает graphwar.v1. */
export function wsProtocols(session: TabSession): string[] | undefined {
  const token = session.get();
  return token ? [WS_PROTOCOL, `bearer.${token}`] : undefined;
}
