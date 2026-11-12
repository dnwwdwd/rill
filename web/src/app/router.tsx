import { lazy } from 'react';
import { createBrowserRouter } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { ProtectedRoute } from '@/app/ProtectedRoute';
import { RootLayout } from '@/app/RootLayout';

const LoginPage = lazy(() => import('@/pages/LoginPage').then((module) => ({ default: module.LoginPage })));
const ReaderPage = lazy(() => import('@/pages/ReaderPage').then((module) => ({ default: module.ReaderPage })));
const FocusPage = lazy(() => import('@/pages/FocusPage').then((module) => ({ default: module.FocusPage })));
const SourceHealthPage = lazy(() => import('@/pages/SourceHealthPage').then((module) => ({ default: module.SourceHealthPage })));
const LegacySourcesPage = lazy(() => import('@/pages/LegacySourcesPage').then((module) => ({ default: module.LegacySourcesPage })));
const SettingsPage = lazy(() => import('@/pages/SettingsPage').then((module) => ({ default: module.SettingsPage })));
const MobileSourcesPage = lazy(() => import('@/pages/MobileSourcesPage').then((module) => ({ default: module.MobileSourcesPage })));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then((module) => ({ default: module.NotFoundPage })));

export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [
      { path: 'login', element: <LoginPage /> },
      {
        element: <ProtectedRoute />,
        children: [
          {
            element: <AppLayout />,
            children: [
              { index: true, element: <ReaderPage /> },
              { path: 'items/:itemId', element: <ReaderPage /> },
              { path: 'items/:itemId/focus', element: <FocusPage /> },
              { path: 'sources', element: <LegacySourcesPage /> },
              { path: 'subscribe', element: <MobileSourcesPage /> },
              { path: 'sources/:sourceId', element: <SourceHealthPage /> },
              { path: 'settings', element: <SettingsPage /> },
              { path: 'settings/:section', element: <SettingsPage /> },
              { path: '*', element: <NotFoundPage /> },
            ],
          },
        ],
      },
    ],
  },
]);
