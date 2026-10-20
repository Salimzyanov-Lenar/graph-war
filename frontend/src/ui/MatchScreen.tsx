import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ClientCommand } from '../contract/schemas';
import { mountFieldScene, type FieldSource } from '../field/fieldScene';
import { selectableFighter } from '../input/fireInput';
import { GameCanvas } from '../scene/GameCanvas';
import { toFieldState } from '../state/gameState';
import type { GameStore } from '../state/gameStore';
import { FirePanel } from './FirePanel';
import { MatchHud } from './MatchHud';
import { useGameState } from './useGameState';

interface MatchScreenProps {
  store: GameStore;
  send(command: ClientCommand): void;
}

export function MatchScreen({ store, send }: MatchScreenProps) {
  const state = useGameState(store);
  const [selectedId, setSelectedId] = useState<string>();

  const selectedRef = useRef(selectedId);
  selectedRef.current = selectedId;
  const selectionListeners = useRef(new Set<() => void>());

  const select = useCallback((id: string | undefined) => {
    setSelectedId(id);
    selectedRef.current = id;
    selectionListeners.current.forEach((l) => l());
  }, []);

  const mount = useMemo(() => {
    const source: FieldSource = {
      getState: () => {
        const current = store.getState();
        return toFieldState(current, selectableFighter(current, selectedRef.current)?.id);
      },
      subscribe: (listener) => {
        const unsubscribe = store.subscribe(listener);
        selectionListeners.current.add(listener);
        return () => {
          unsubscribe();
          selectionListeners.current.delete(listener);
        };
      },
      onFighterClick: (fighter) => {
        if (selectableFighter(store.getState(), fighter.id)) select(fighter.id);
      },
    };
    return mountFieldScene(source);
  }, [store, select]);

  // Выбор живёт только пока боец жив и принадлежит игроку; сцена перерисуется по подписке store.
  useEffect(() => {
    if (selectedId && !selectableFighter(state, selectedId)) select(undefined);
  }, [state, selectedId, select]);

  return (
    <div className="match-wrap">
      <MatchHud state={state} send={send} />
      <div className="match">
        <GameCanvas mount={mount} />
        <FirePanel state={state} selectedId={selectedId} onSelect={select} send={send} />
      </div>
    </div>
  );
}
