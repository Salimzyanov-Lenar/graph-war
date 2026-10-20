import { PROTOCOL_VERSION, type ClientCommand } from '../contract/schemas';
import type { Transport, TransportListener } from './transport';

export interface SocketLike {
  onopen: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  readyState: number;
  send(data: string): void;
  close(): void;
}

export type SocketFactory = (url: string, protocols?: string[]) => SocketLike;

const OPEN = 1;

/** Реальный WebSocket за интерфейсом Transport. Адрес относительный к текущему origin; личность — токен вкладки в subprotocol (или cookie). */
export function createWsTransport(
  url: string,
  createSocket: SocketFactory = (u, protocols) => new WebSocket(u, protocols),
  protocols: () => string[] | undefined = () => undefined,
): Transport {
  const listeners = new Set<TransportListener>();
  let socket: SocketLike | undefined;

  const detach = (target: SocketLike) => {
    target.onopen = target.onclose = target.onmessage = target.onerror = null;
  };

  return {
    connect() {
      if (socket) return;
      const current = createSocket(url, protocols());
      socket = current;
      current.onopen = () => listeners.forEach((l) => l.onOpen());
      current.onmessage = (event) => listeners.forEach((l) => l.onMessage(event.data));
      current.onclose = (event) => {
        detach(current);
        if (socket === current) socket = undefined;
        listeners.forEach((l) => l.onClose(event?.code));
      };
      current.onerror = () => {};
    },
    close() {
      const current = socket;
      if (!current) return;
      socket = undefined;
      detach(current);
      current.close();
      listeners.forEach((l) => l.onClose());
    },
    send(command: ClientCommand) {
      if (!socket || socket.readyState !== OPEN) return;
      socket.send(JSON.stringify({ v: PROTOCOL_VERSION, ...command }));
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}

export function defaultWsUrl(location: Pick<Location, 'protocol' | 'host'> = window.location): string {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/ws`;
}
