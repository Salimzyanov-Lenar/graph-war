import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { SERVER_STATUS_TEXT, checkServerStatus, type ServerStatus } from '../api/health';
import { ApiError, createRoomsApi, inviteLink, readInvite, type Guest, type Room, type RoomsApi } from '../api/rooms';
import type { ClientCommand } from '../contract/schemas';
import { createGameStore, type GameStore } from '../state/gameStore';
import { createWsTransport, defaultWsUrl } from '../transport/wsTransport';
import { MatchScreen } from './MatchScreen';
import { useGameState } from './useGameState';

const API_ERRORS: Record<string, string> = {
  room_full: 'В комнате уже два игрока.',
  room_expired: 'Комната закрыта или приглашение устарело.',
  unauthorized: 'Сессия не найдена — введите имя заново.',
  rate_limited: 'Слишком часто. Подождите немного.',
  invalid_invite: 'Приглашение не найдено. Попросите у друга новую ссылку.',
  invalid_name: 'Имя не подходит. Попробуйте другое.',
  name_too_long: 'Имя слишком длинное (не больше 24 символов).',
};

export function apiErrorText(error: unknown): string {
  if (error instanceof ApiError) {
    return (error.code && API_ERRORS[error.code]) || `Сервер ответил ошибкой (${error.status}).`;
  }
  return 'Не удалось связаться с сервером. Он запущен?';
}

const defaultCheckStatus = () => checkServerStatus();

interface Joined {
  roomId: string;
  invite?: string;
}

interface OnlineAppProps {
  api?: RoomsApi;
  search?: string;
  origin?: string;
  checkStatus?: () => Promise<ServerStatus>;
}

export function OnlineApp({
  api: apiProp,
  search = window.location.search,
  origin = window.location.origin + window.location.pathname,
  checkStatus = defaultCheckStatus,
}: OnlineAppProps) {
  const api = useMemo(() => apiProp ?? createRoomsApi(), [apiProp]);
  const invite = useMemo(() => readInvite(search), [search]);
  const [name, setName] = useState('');
  const [guest, setGuest] = useState<Guest>();
  const [joined, setJoined] = useState<Joined>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<ServerStatus>('checking');

  useEffect(() => {
    let active = true;
    void checkStatus().then((next) => active && setStatus(next));
    return () => {
      active = false;
    };
  }, [checkStatus]);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError(undefined);
    try {
      await action();
    } catch (failure) {
      setError(apiErrorText(failure));
    } finally {
      setBusy(false);
    }
  };

  const enter = (event: FormEvent) => {
    event.preventDefault();
    const displayName = name.trim();
    if (!displayName) return;
    void run(async () => {
      const created = await api.createGuest(displayName);
      if (invite) await api.joinRoom(invite.roomId, invite.inviteId);
      setGuest(created);
      if (invite) setJoined({ roomId: invite.roomId });
    });
  };

  const createRoom = () =>
    run(async () => {
      const room: Room = await api.createRoom();
      setJoined({ roomId: room.roomId, invite: inviteLink(origin, room) });
    });

  if (!guest) {
    return (
      <form className="lobby" onSubmit={enter}>
        <h2>{invite ? 'Вас пригласили в игру' : 'Новая игра'}</h2>
        <label htmlFor="display-name">Ваше имя</label>
        <input
          id="display-name"
          value={name}
          maxLength={24}
          autoComplete="nickname"
          onChange={(event) => setName(event.target.value)}
        />
        <button type="submit" disabled={busy || !name.trim()}>
          {invite ? 'Войти в комнату' : 'Продолжить'}
        </button>
        {error && <p role="alert">{error}</p>}
        <p className="fire-status" role="status">
          {SERVER_STATUS_TEXT[status]}
        </p>
      </form>
    );
  }

  if (!joined) {
    return (
      <div className="lobby">
        <h2>Привет, {guest.displayName}</h2>
        <button type="button" disabled={busy} onClick={() => void createRoom()}>
          Создать комнату
        </button>
        {error && <p role="alert">{error}</p>}
      </div>
    );
  }

  return <RoomSession api={api} guest={guest} joined={joined} onLeave={() => setJoined(undefined)} />;
}

function RoomSession({ api, guest, joined, onLeave }: { api: RoomsApi; guest: Guest; joined: Joined; onLeave(): void }) {
  const [session, setSession] = useState<{ store: GameStore; send(command: ClientCommand): void }>();

  useEffect(() => {
    const transport = createWsTransport(defaultWsUrl());
    const store = createGameStore(transport);
    store.connect();
    setSession({ store, send: (command) => transport.send(command) });
    return () => {
      store.dispose();
      transport.close();
      setSession(undefined);
    };
  }, []);

  const leave = () => {
    void api.leaveRoom(joined.roomId).catch(() => {});
    onLeave();
  };

  if (!session) return null;
  return <RoomView api={api} guest={guest} joined={joined} session={session} onLeave={leave} />;
}

function RoomView({
  api,
  guest,
  joined,
  session,
  onLeave,
}: {
  api: RoomsApi;
  guest: Guest;
  joined: Joined;
  session: { store: GameStore; send(command: ClientCommand): void };
  onLeave(): void;
}) {
  const state = useGameState(session.store);
  const [error, setError] = useState<string>();
  const me = state.players.find((player) => player.guestId === guest.guestId);

  if (state.phase === 'active' || state.phase === 'resolving' || state.phase === 'finished') {
    return (
      <>
        <MatchScreen store={session.store} send={session.send} />
        <button type="button" onClick={onLeave}>
          Выйти из комнаты
        </button>
      </>
    );
  }

  if (state.phase === 'aborted') {
    return (
      <div className="lobby">
        <p role="status">Матч прерван.</p>
        <button type="button" onClick={onLeave}>
          Выйти из комнаты
        </button>
      </div>
    );
  }

  const toggleReady = () => {
    setError(undefined);
    api.setReady(joined.roomId, !me?.ready).catch((failure: unknown) => setError(apiErrorText(failure)));
  };

  return (
    <div className="lobby">
      <h2>Комната</h2>
      {joined.invite && (
        <p>
          Отправьте другу ссылку: <input readOnly value={joined.invite} aria-label="Ссылка-приглашение" onFocus={(event) => event.target.select()} />
        </p>
      )}
      <p className="fire-status">
        Соединение: {state.connection === 'open' ? 'есть' : state.connection === 'connecting' ? 'устанавливается…' : 'нет'}
      </p>
      <ul>
        {state.players.map((player) => (
          <li key={player.guestId}>
            {player.displayName} (сторона {player.side}) — {player.ready ? 'готов' : 'не готов'}
          </li>
        ))}
        {state.players.length < 2 && <li>Ждём второго игрока…</li>}
      </ul>
      <button type="button" disabled={!me || state.players.length < 2} onClick={toggleReady}>
        {me?.ready ? 'Отменить готовность' : 'Готов'}
      </button>
      <button type="button" onClick={onLeave}>
        Выйти из комнаты
      </button>
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
