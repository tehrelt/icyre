import type { z } from 'zod';

import { env } from '@/shared/config/env';

import { ApiError, ErrorBodySchema } from './errors';

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
  body?: unknown;
  signal?: AbortSignal;
}

/**
 * Calls the public API (/api/v1 behind the gateway) and validates the
 * response with a zod schema. The browser never talks to services directly.
 */
export async function apiRequest<S extends z.ZodType>(path: string, schema: S, opts: RequestOptions = {}): Promise<z.output<S>> {
  const method = opts.method ?? 'GET';
  const { status, json } = env.useMocks
    ? // Loaded lazily so fixtures never ship in a production bundle that talks to the real API.
      await (await import('./mock/server')).mockRequest(method, path, opts.body, opts.signal)
    : await httpRequest(method, path, opts);

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

async function httpRequest(method: string, path: string, opts: RequestOptions): Promise<{ status: number; json: unknown }> {
  const res = await fetch(env.apiBaseUrl + path, {
    method,
    signal: opts.signal,
    credentials: 'include',
    headers: {
      Accept: 'application/json',
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
