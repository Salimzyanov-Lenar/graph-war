export type Side = 'A' | 'B';

export interface Obstacle {
  x: number;
  y: number;
  r: number;
}

export interface Fighter {
  id: string;
  side: Side;
  x: number;
  y: number;
  alive: boolean;
}

/** Последний выстрел так, как его прислал сервер. `live` — выстрел только что принят (фаза resolving), его надо проиграть. */
export interface ShotView {
  shotId: string;
  points: readonly (readonly [number, number])[];
  stopReason: string;
  destroyedFighterId?: string;
  live: boolean;
}

export interface FieldState {
  width: number;
  height: number;
  obstacles: readonly Obstacle[];
  fighters: readonly Fighter[];
  selectedFighterId?: string;
  shot?: ShotView;
}
