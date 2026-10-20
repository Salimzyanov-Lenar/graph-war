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

export interface FieldState {
  width: number;
  height: number;
  obstacles: readonly Obstacle[];
  fighters: readonly Fighter[];
  selectedFighterId?: string;
}
