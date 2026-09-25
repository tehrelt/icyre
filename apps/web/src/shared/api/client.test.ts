import { z } from 'zod';

import { apiGet } from './client';
import { ApiError } from './errors';
import { mockAuth } from './mock/server';
import { getAccessToken, resetSessionForTests } from './session';

beforeEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = true;
  mockAuth.tokens.clear();
});

describe('apiGet (mock API)', () => {
  it('validates responses with zod', async () => {
    const me = await apiGet('/users/me', z.object({ displayName: z.string() }));
    expect(me.displayName).toBe('Rin Aoki');
  });

  it('maps the error envelope to ApiError', async () => {
    const err = await apiGet('/pages/albums/nope', z.unknown()).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: 'ALBUM_NOT_FOUND' });
    expect((err as ApiError).requestId).toBeTruthy();
  });

  it('rejects unexpected response shapes', async () => {
    const err = await apiGet('/users/me', z.object({ id: z.number() })).catch((e: unknown) => e);
    expect(err).toMatchObject({ code: 'INVALID_RESPONSE' });
  });
});

describe('session', () => {
  const Me = z.object({ id: z.string() });

  it('restores the session from the refresh cookie once', async () => {
    await Promise.all([apiGet('/users/me', Me), apiGet('/users/me', Me), apiGet('/pages/home', z.unknown())]);
    expect(mockAuth.issued).toBeGreaterThan(0);
    const issued = mockAuth.issued;
    await apiGet('/users/me', Me);
    expect(mockAuth.issued).toBe(issued);
    expect(getAccessToken()).toBe(`mock-access-${issued}`);
  });

  it('refreshes once and retries when the access token expires', async () => {
    await apiGet('/users/me', Me);
    const before = getAccessToken();
    mockAuth.tokens.clear(); // access token expired server-side
    await expect(apiGet('/users/me', Me)).resolves.toMatchObject({ id: 'usr-rin' });
    expect(getAccessToken()).not.toBe(before);
  });

  it('treats a missing refresh cookie as an anonymous visitor', async () => {
    mockAuth.signedIn = false;
    const err = await apiGet('/users/me', Me).catch((e: unknown) => e);
    expect(err).toMatchObject({ status: 401, code: 'UNAUTHENTICATED' });
    expect(getAccessToken()).toBeNull();
    // Public pages keep working without a session.
    await expect(apiGet('/pages/home', z.unknown())).resolves.toBeTruthy();
  });

  it('gives up after one refresh when the session was revoked', async () => {
    await apiGet('/users/me', Me);
    mockAuth.tokens.clear();
    mockAuth.signedIn = false;
    await expect(apiGet('/users/me', Me)).rejects.toMatchObject({ status: 401 });
    expect(getAccessToken()).toBeNull();
  });
});
