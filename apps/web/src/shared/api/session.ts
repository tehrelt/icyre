import { z } from 'zod';

/**
 * Browser side of the Auth contract (EPIC-011):
 * - the access token lives only in memory (never in storage);
 * - the refresh token is an HttpOnly cookie scoped to /api/v1/auth, so
 *   JavaScript never sees it — `POST /auth/refresh` rotates it;
 * - on start the session is restored with one refresh call, and a 401 on
 *   any request triggers at most one refresh + retry.
 */
export const TokenResponseSchema = z.object({
  accessToken: z.string(),
  tokenType: z.literal('Bearer'),
  expiresIn: z.number(),
  expiresAt: z.string(),
  user: z.object({ id: z.string(), email: z.string(), roles: z.array(z.string()) }),
});

export type TokenResponse = z.infer<typeof TokenResponseSchema>;

type Refresher = () => Promise<TokenResponse | null>;

let accessToken: string | null = null;
let bootstrapped = false;
let inflight: Promise<string | null> | null = null;
const listeners = new Set<() => void>();

export const getAccessToken = () => accessToken;

/** Stores a token from login/register/refresh. */
export function setSession(res: TokenResponse | null): void {
  accessToken = res?.accessToken ?? null;
  bootstrapped = true;
  listeners.forEach((l) => l());
}

export const clearSession = () => setSession(null);

/** Called whenever the session changes (sign-in, refresh failure, logout). */
export function onSessionChange(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/**
 * Single-flight refresh: concurrent 401s share one rotation, otherwise the
 * second call would present an already-rotated token and trip reuse detection.
 */
export function refreshSession(refresh: Refresher): Promise<string | null> {
  inflight ??= refresh()
    .then((res) => {
      setSession(res);
      return accessToken;
    })
    .catch(() => {
      setSession(null);
      return null;
    })
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

/** First request of the page: restore the session from the refresh cookie once. */
export async function ensureSession(refresh: Refresher): Promise<string | null> {
  if (!bootstrapped) return refreshSession(refresh);
  return inflight ?? accessToken;
}

/** Test hook: forget everything, as after a page reload. */
export function resetSessionForTests(): void {
  accessToken = null;
  bootstrapped = false;
  inflight = null;
}
