import type { z } from 'zod';

import { env } from '@/shared/config/env';

import { ApiError, ErrorBodySchema } from './errors';
import { ensureSession, getAccessToken, refreshSession, TokenResponseSchema, type TokenResponse } from './session';

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
  body?: unknown;
  signal?: AbortSignal;
}

interface RawResponse {
  status: number;
  json: unknown;
}

// Auth endpoints manage the session themselves: no bearer, no refresh loop.
const isAuthPath = (path: string) => path.startsWith('/auth/');

async function send(method: string, path: string, opts: RequestOptions, token: string | null): Promise<RawResponse> {
  return env.useMocks
    ? // Loaded lazily so fixtures never ship in a production bundle that talks to the real API.
      (await import('./mock/server')).mockRequest(method, path, opts.body, opts.signal, token)
    : httpRequest(method, path, opts, token);
}

/** POST /auth/refresh with the HttpOnly cookie; null when there is no session. */
async function refreshCall(): Promise<TokenResponse | null> {
  const { status, json } = await send('POST', '/auth/refresh', {}, null);
  if (status !== 200) return null;
  const parsed = TokenResponseSchema.safeParse(json);
  return parsed.success ? parsed.data : null;
}

/**
 * Calls the public API (/api/v1 behind the gateway) and validates the
 * response with a zod schema. The browser never talks to services directly.
 */
export async function apiRequest<S extends z.ZodType>(path: string, schema: S, opts: RequestOptions = {}): Promise<z.output<S>> {
  const method = opts.method ?? 'GET';
  const auth = !isAuthPath(path);
  const token = auth ? await ensureSession(refreshCall) : null;
  let { status, json } = await send(method, path, opts, token);
  // Expired or revoked access token: rotate once and retry.
  if (status === 401 && auth && token) {
    // Another request may have rotated the session meanwhile: reuse its token.
    const current = getAccessToken();
    const next = current && current !== token ? current : await refreshSession(refreshCall);
    if (next) ({ status, json } = await send(method, path, opts, next));
  }

  if (status >= 400) {
    const parsed = ErrorBodySchema.safeParse(json);
    if (parsed.success) {
      const e = parsed.data.error;
      throw new ApiError(status, e.code, e.message, e.requestId, e.details);
    }
    throw new ApiError(status, 'HTTP_ERROR', `Request failed with status ${status}`);
  }

  const result = schema.safeParse(json);
  if (!result.success) {
    throw new ApiError(status, 'INVALID_RESPONSE', `Unexpected response shape for ${path}: ${result.error.message}`);
  }
  return result.data;
}

export const apiGet = <S extends z.ZodType>(path: string, schema: S, signal?: AbortSignal) => apiRequest(path, schema, { signal });

async function httpRequest(method: string, path: string, opts: RequestOptions, token: string | null): Promise<RawResponse> {
  const res = await fetch(env.apiBaseUrl + path, {
    method,
    signal: opts.signal,
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(opts.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
  const text = await res.text();
  let json: unknown = null;
  if (text) {
    try {
      json = JSON.parse(text);
    } catch {
      throw new ApiError(res.status, 'INVALID_RESPONSE', 'Response is not JSON');
    }
  }
  return { status: res.status, json };
}
