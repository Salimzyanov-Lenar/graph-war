import type { ClientCommand } from '../contract/schemas';

export interface TransportListener {
  onOpen(): void;
  /** `code` — код закрытия WebSocket, если он известен (4001 — сессию заняла другая вкладка). */
  onClose(code?: number): void;
  /** Сырое сообщение: строка JSON (WebSocket) или уже разобранное значение (мок). Проверяет его слой состояния. */
  onMessage(raw: unknown): void;
}

/** Общий интерфейс для мока и будущего HTTP/WebSocket-транспорта (F05+). */
export interface Transport {
  connect(): void;
  close(): void;
  send(command: ClientCommand): void;
  subscribe(listener: TransportListener): () => void;
}
