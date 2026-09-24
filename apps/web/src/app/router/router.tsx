import { createBrowserRouter, type RouteObject } from 'react-router';

import { HomePage } from '@/pages/home';
import { NotBuiltPage } from '@/pages/not-built/NotBuiltPage';

import { AppShell } from '../layouts/AppShell';

export const appRoutes: RouteObject[] = [
  {
    element: <AppShell />,
    children: [
      { index: true, element: <HomePage /> },
      // Screens from the canvas planned in the next frontend epics.
      { path: '*', element: <NotBuiltPage /> },
    ],
  },
];

export const createAppRouter = () => createBrowserRouter(appRoutes);
