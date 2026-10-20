import { useSyncExternalStore } from 'react';
import type { GameState } from '../state/gameState';
import type { GameStore } from '../state/gameStore';

export function useGameState(store: GameStore): GameState {
  return useSyncExternalStore(store.subscribe, store.getState);
}
