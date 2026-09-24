import { render } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router';

import { AppProviders } from '@/app/providers/AppProviders';
import { appRoutes } from '@/app/router/router';
import { initialPlayerState, usePlayerStore } from '@/features/player/model/playerStore';

/** Renders the whole app (shell + routes + providers) at `path`. */
export function renderApp(path = '/') {
  usePlayerStore.setState(initialPlayerState);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(appRoutes, { initialEntries: [path] });
  const utils = render(
    <AppProviders queryClient={queryClient}>
      <RouterProvider router={router} />
    </AppProviders>,
  );
  return { ...utils, router, queryClient };
}
