import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';

/** GET /users/me (User Profile Service, EPIC-012). */
export const CurrentUserSchema = z.object({
  id: z.string(),
  username: z.string(),
  displayName: z.string(),
  avatarUrl: z.string().nullable().default(null),
  country: z.string().default(''),
  language: z.string().default(''),
  // Placeholder artwork index for the avatar while there is no avatarUrl.
  art: z.number().int().default(0),
});

export type CurrentUser = z.infer<typeof CurrentUserSchema>;

export const currentUserQuery = queryOptions({
  queryKey: ['me'],
  queryFn: ({ signal }) => apiGet('/users/me', CurrentUserSchema, signal),
  staleTime: 5 * 60_000,
  // 401 means an anonymous visitor (the client already tried a refresh).
  retry: (failures, error) => !(error instanceof ApiError && error.status === 401) && failures < 2,
});

export const useCurrentUser = () => useQuery(currentUserQuery);
