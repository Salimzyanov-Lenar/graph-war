import type { Fighter } from './types';
import type { Point } from './viewport';

export const FIGHTER_RADIUS = 1.2;

/** Работает в мировых координатах, поэтому не зависит от размера окна и DPR. */
export function pickFighter(
  fighters: readonly Fighter[],
  world: Point,
  radius = FIGHTER_RADIUS,
): Fighter | undefined {
  let best: Fighter | undefined;
  let bestDistance = Infinity;
  for (const fighter of fighters) {
    const distance = Math.hypot(fighter.x - world.x, fighter.y - world.y);
    if (distance <= radius && distance < bestDistance) {
      best = fighter;
      bestDistance = distance;
    }
  }
  return best;
}
