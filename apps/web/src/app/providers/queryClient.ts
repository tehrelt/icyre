import { QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/shared/api/errors';

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        // Client errors (4xx) are final; retry network and 5xx once.
        retry: (count, error) => !(error instanceof ApiError && error.status < 500) && count < 1,
      },
    },
  });
}
