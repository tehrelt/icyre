import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/shared/api/client';

export const CurrentUserSchema = z.object({
  id: z.string(),
  displayName: z.string(),
  avatarUrl: z.string().nullable().default(null),
  art: z.number().int().default(0),
});

export type CurrentUser = z.infer<typeof CurrentUserSchema>;

export const currentUserQuery = queryOptions({
  queryKey: ['me'],
  queryFn: ({ signal }) => apiGet('/me', CurrentUserSchema, signal),
  staleTime: 5 * 60_000,
});

export const useCurrentUser = () => useQuery(currentUserQuery);
