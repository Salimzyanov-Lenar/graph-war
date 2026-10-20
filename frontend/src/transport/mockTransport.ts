import type { ClientCommand } from '../contract/schemas';
import type { Transport, TransportListener } from './transport';

export class MockTransport implements Transport {
  readonly sent: ClientCommand[] = [];
  private listeners = new Set<TransportListener>();

  constructor(private readonly onSend?: (command: ClientCommand) => void) {}

  connect() {
    queueMicrotask(() => this.open());
  }

  close() {
    this.drop();
  }

  send(command: ClientCommand) {
    this.sent.push(command);
    this.onSend?.(command);
  }

  subscribe(listener: TransportListener) {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  open() {
    this.listeners.forEach((l) => l.onOpen());
  }

  drop() {
    this.listeners.forEach((l) => l.onClose());
  }

  /** Отдаёт «серверное» сообщение так же, как это сделал бы сокет: без предварительной проверки. */
  emit(raw: unknown) {
    this.listeners.forEach((l) => l.onMessage(raw));
  }
}
