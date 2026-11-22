import { ApiError } from '@/shared/api/errors';

import { mapAuthError } from './authApi';

describe('mapAuthError', () => {
  it('maps VALIDATION_FAILED field errors', () => {
    const err = new ApiError(422, 'VALIDATION_FAILED', 'Request validation failed', undefined, {
      fields: { email: 'must be a valid email address', password: 'must be at least 10 characters' },
    });
    expect(mapAuthError(err)).toEqual({
      fieldErrors: {
        email: 'must be a valid email address',
        password: 'must be at least 10 characters',
      },
    });
  });

  it('maps EMAIL_TAKEN to email field error', () => {
    const err = new ApiError(409, 'EMAIL_TAKEN', 'An account with this email already exists');
    expect(mapAuthError(err)).toEqual({
      fieldErrors: { email: 'An account with this email already exists.' },
    });
  });

  it('maps INVALID_CREDENTIALS and TOO_MANY_ATTEMPTS to form errors', () => {
    expect(mapAuthError(new ApiError(401, 'INVALID_CREDENTIALS', 'bad'))).toEqual({
      formError: 'Email or password is incorrect.',
    });
    expect(mapAuthError(new ApiError(429, 'TOO_MANY_ATTEMPTS', 'slow down'))).toEqual({
      formError: 'Too many sign-in attempts. Try again in a few minutes.',
    });
  });

  it('maps unknown errors to a network fallback message', () => {
    expect(mapAuthError(new Error('Network down'))).toEqual({
      formError: 'Unable to reach ICYRE right now. Try again.',
    });
  });
});
