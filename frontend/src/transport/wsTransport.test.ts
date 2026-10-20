import { describe, expect, it, vi } from 'vitest';
import { createWsTransport, defaultWsUrl, type SocketLike } from './wsTransport';

class FakeSocket implements SocketLike {
  onopen: SocketLike['onopen'] = null;
  onclose: SocketLike['onclose'] = null;
  onmessage: SocketLike['onmessage'] = null;
  onerror: SocketLike['onerror'] = null;
  readyState = 0;
  sent: string[] = [];
  closed = false;
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.closed = true;
  }
  open() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }
  receive(data: unknown) {
    this.onmessage?.({ data } as MessageEvent);
  }
  drop(code?: number) {
    this.readyState = 3;
    this.onclose?.({ code } as CloseEvent);
  }
}

function setup() {
  const sockets: FakeSocket[] = [];
  const transport = createWsTransport('ws://h/ws', () => {
    const socket = new FakeSocket();
    sockets.push(socket);
    return socket;
  });
  const listener = { onOpen: vi.fn(), onClose: vi.fn(), onMessage: vi.fn() };
  transport.subscribe(listener);
  return { sockets, transport, listener };
}

describe('createWsTransport', () => {
  it('передаёт open, сырые сообщения и close слушателям', () => {
    const { sockets, transport, listener } = setup();
    transport.connect();
    sockets[0]!.open();
    sockets[0]!.receive('{"type":"snapshot"}');
    sockets[0]!.drop();

    expect(listener.onOpen).toHaveBeenCalledTimes(1);
    expect(listener.onMessage).toHaveBeenCalledWith('{"type":"snapshot"}');
    expect(listener.onClose).toHaveBeenCalledTimes(1);
  });

  it('передаёт код закрытия и subprotocol с токеном вкладки', () => {
    const calls: (string[] | undefined)[] = [];
    const sockets: FakeSocket[] = [];
    const transport = createWsTransport(
      'ws://h/ws',
      (_url, protocols) => {
        calls.push(protocols);
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      () => ['graphwar.v1', 'bearer.tok'],
    );
    const onClose = vi.fn();
    transport.subscribe({ onOpen: vi.fn(), onClose, onMessage: vi.fn() });
    transport.connect();
    sockets[0]!.drop(4001);
    expect(calls).toEqual([['graphwar.v1', 'bearer.tok']]);
    expect(onClose).toHaveBeenCalledWith(4001);
  });

  it('повторный connect не открывает второй сокет', () => {
    const { sockets, transport } = setup();
    transport.connect();
    transport.connect();
    expect(sockets).toHaveLength(1);
  });

  it('send добавляет версию протокола и молчит, пока сокет не открыт', () => {
    const { sockets, transport } = setup();
    transport.connect();
    transport.send({ type: 'room.ready', payload: { ready: true } });
    expect(sockets[0]!.sent).toEqual([]);

    sockets[0]!.open();
    transport.send({ type: 'room.ready', payload: { ready: true } });
    expect(JSON.parse(sockets[0]!.sent[0]!)).toEqual({ v: 1, type: 'room.ready', payload: { ready: true } });
  });

  it('close закрывает сокет один раз и сообщает слушателю; после него можно подключиться снова', () => {
    const { sockets, transport, listener } = setup();
    transport.connect();
    sockets[0]!.open();
    transport.close();
    transport.close();

    expect(sockets[0]!.closed).toBe(true);
    expect(listener.onClose).toHaveBeenCalledTimes(1);

    transport.connect();
    expect(sockets).toHaveLength(2);
  });

  it('адрес строится от текущего origin', () => {
    expect(defaultWsUrl({ protocol: 'http:', host: '127.0.0.1:5173' })).toBe('ws://127.0.0.1:5173/ws');
    expect(defaultWsUrl({ protocol: 'https:', host: 'game.example' })).toBe('wss://game.example/ws');
  });
});
