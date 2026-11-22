import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';

import { apiRequest } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { clearSession, setSession, TokenResponseSchema, type TokenResponse } from '@/shared/api/session';

export interface CredentialsPayload {
  email: string;
  password: string;
}

export function loginRequest(payload: CredentialsPayload): Promise<TokenResponse> {
  return apiRequest('/auth/login', TokenResponseSchema, {
    method: 'POST',
    body: { email: payload.email.trim().toLowerCase(), password: payload.password },
  });
}

export function registerRequest(payload: CredentialsPayload): Promise<TokenResponse> {
  return apiRequest('/auth/register', TokenResponseSchema, {
    method: 'POST',
    body: { email: payload.email.trim().toLowerCase(), password: payload.password },
  });
}

export function logoutRequest(): Promise<null> {
  return apiRequest('/auth/logout', z.null(), { method: 'POST' });
}

export interface MappedAuthError {
  formError?: string;
  fieldErrors?: Partial<Record<'email' | 'password', string>>;
}

export function mapAuthError(error: unknown): MappedAuthError {
  if (!(error instanceof ApiError)) {
    return { formError: 'Unable to reach ICYRE right now. Try again.' };
  }
  if (error.code === 'VALIDATION_FAILED') {
    const rawFields = error.details?.fields;
    if (rawFields && typeof rawFields === 'object') {
      const fields = rawFields as Record<string, unknown>;
      const fieldErrors: Partial<Record<'email' | 'password', string>> = {};
      if (typeof fields.email === 'string') fieldErrors.email = fields.email;
      if (typeof fields.password === 'string') fieldErrors.password = fields.password;
      if (fieldErrors.email || fieldErrors.password) return { fieldErrors };
    }
    return { formError: 'Check the highlighted fields and try again.' };
  }
  if (error.code === 'EMAIL_TAKEN') {
    return { fieldErrors: { email: 'An account with this email already exists.' } };
  }
  if (error.code === 'INVALID_CREDENTIALS') {
    return { formError: 'Email or password is incorrect.' };
  }
  if (error.code === 'TOO_MANY_ATTEMPTS') {
    return { formError: 'Too many sign-in attempts. Try again in a few minutes.' };
  }
  return { formError: error.message || 'Something went wrong. Try again.' };
}

export function useLoginMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: loginRequest,
    onSuccess: async (res) => {
      setSession(res);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['me'] }),
        queryClient.invalidateQueries({ queryKey: ['library'] }),
        queryClient.invalidateQueries({ queryKey: ['playlists'] }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
      ]);
    },
  });
}

export function useRegisterMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: registerRequest,
    onSuccess: async (res) => {
      setSession(res);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['me'] }),
        queryClient.invalidateQueries({ queryKey: ['library'] }),
        queryClient.invalidateQueries({ queryKey: ['playlists'] }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
      ]);
    },
  });
}

export function useLogoutMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: logoutRequest,
    onSettled: async () => {
      clearSession();
      queryClient.removeQueries({ queryKey: ['me'] });
      queryClient.removeQueries({ queryKey: ['library'] });
      queryClient.removeQueries({ queryKey: ['playlists'] });
      await queryClient.invalidateQueries({ queryKey: ['home'] });
    },
  });
}
