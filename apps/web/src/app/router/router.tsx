import { createBrowserRouter, type RouteObject } from 'react-router';

import { AlbumPage } from '@/pages/album';
import { AuthPage } from '@/pages/auth';
import { HomePage } from '@/pages/home';
import { LibraryPage } from '@/pages/library';
import { SearchPage } from '@/pages/search';
import { NotBuiltPage } from '@/pages/not-built/NotBuiltPage';

import { AppShell } from '../layouts/AppShell';

export const appRoutes: RouteObject[] = [
  { path: 'login', element: <AuthPage mode="login" /> },
  { path: 'register', element: <AuthPage mode="register" /> },
  {
    element: <AppShell />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'album/:albumId', element: <AlbumPage /> },
      { path: 'search', element: <SearchPage /> },
      { path: 'library', element: <LibraryPage mode="library" /> },
      { path: 'liked', element: <LibraryPage mode="liked" /> },
      // Screens from the canvas planned in the next frontend epics.
      { path: '*', element: <NotBuiltPage /> },
    ],
  },
];

export const createAppRouter = () => createBrowserRouter(appRoutes);
