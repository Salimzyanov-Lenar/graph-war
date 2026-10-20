export interface ReconnectPolicy {
  baseMs: number;
  maxMs: number;
  maxAttempts: number;
}

export const DEFAULT_RECONNECT: ReconnectPolicy = { baseMs: 500, maxMs: 8000, maxAttempts: 8 };

/** Экспоненциальная задержка с «полным» jitter: от половины до целого значения. `attempt` считается с 1. */
export function backoffDelayMs(attempt: number, policy: ReconnectPolicy = DEFAULT_RECONNECT, random: () => number = Math.random): number {
  const exp = Math.min(policy.maxMs, policy.baseMs * 2 ** Math.max(0, attempt - 1));
  return Math.round(exp / 2 + (exp / 2) * random());
}
