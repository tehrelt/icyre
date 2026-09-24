import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { PlayerProvider } from '@/features/player';
import { createQueryClient } from './queryClient';

/** Server state (TanStack Query) and the global player. */
export function AppProviders({ children, queryClient }: { children: ReactNode; queryClient?: QueryClient }) {
  const [client] = useState(() => queryClient ?? createQueryClient());
  return (
    <QueryClientProvider client={client}>
      <PlayerProvider>{children}</PlayerProvider>
    </QueryClientProvider>
  );
}
