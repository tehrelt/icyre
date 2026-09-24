import { createBrowserRouter, type RouteObject } from 'react-router';

import { AlbumPage } from '@/pages/album';
import { HomePage } from '@/pages/home';
import { SearchPage } from '@/pages/search';
import { NotBuiltPage } from '@/pages/not-built/NotBuiltPage';

import { AppShell } from '../layouts/AppShell';

export const appRoutes: RouteObject[] = [
  {
    element: <AppShell />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'album/:albumId', element: <AlbumPage /> },
      { path: 'search', element: <SearchPage /> },
      // Screens from the canvas planned in the next frontend epics.
      { path: '*', element: <NotBuiltPage /> },
    ],
  },
];

export const createAppRouter = () => createBrowserRouter(appRoutes);
