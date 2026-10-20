import type { ClientCommand } from '../contract/schemas';

export interface TransportListener {
  onOpen(): void;
  onClose(): void;
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
