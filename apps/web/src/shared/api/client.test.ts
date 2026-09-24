import { z } from 'zod';

import { apiGet } from './client';
import { ApiError } from './errors';

describe('apiGet (mock API)', () => {
  it('validates responses with zod', async () => {
    const me = await apiGet('/me', z.object({ displayName: z.string() }));
    expect(me.displayName).toBe('Rin Aoki');
  });

  it('maps the error envelope to ApiError', async () => {
    const err = await apiGet('/pages/albums/nope', z.unknown()).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: 'ALBUM_NOT_FOUND' });
    expect((err as ApiError).requestId).toBeTruthy();
  });

  it('rejects unexpected response shapes', async () => {
    const err = await apiGet('/me', z.object({ id: z.number() })).catch((e: unknown) => e);
    expect(err).toMatchObject({ code: 'INVALID_RESPONSE' });
  });
});
