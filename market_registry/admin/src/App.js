import React, { Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, Spin, theme as antdTheme } from 'antd';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import MainLayout from './components/MainLayout';
import { APP_BASENAME } from './config/runtime';
import { useI18n } from './i18n';
import { useThemeMode } from './theme';

function lazyPage(loader) {
  const Component = React.lazy(loader);
  Component.preload = loader;
  return Component;
}

const Login = lazyPage(() => import('./pages/Login'));
const Dashboard = lazyPage(() => import('./pages/Dashboard'));
const Publish = lazyPage(() => import('./pages/Publish'));
const Artifacts = lazyPage(() => import('./pages/Artifacts'));
const Settings = lazyPage(() => import('./pages/Settings'));

function PrivateRoute({ children }) {
  const token = localStorage.getItem('token');
  return token ? children : <Navigate to="/login" />;
}

function RouteFallback({ fullscreen = false }) {
  const { t } = useI18n();
  return (
    <div className={fullscreen ? 'market-route-loading is-fullscreen' : 'market-route-loading'}>
      <Spin size="large" />
      <span>{t('加载中...')}</span>
    </div>
  );
}

function RouteSuspense({ children, fullscreen = false }) {
  return (
    <Suspense fallback={<RouteFallback fullscreen={fullscreen} />}>
      {children}
    </Suspense>
  );
}

export default function App() {
  const { locale } = useI18n();
  const { themeMode } = useThemeMode();
  const isDark = themeMode === 'dark';
  const preloadAdminRoutes = React.useCallback(() => {
    Dashboard.preload?.();
    Publish.preload?.();
    Artifacts.preload?.();
    Settings.preload?.();
  }, []);
  const themeConfig = React.useMemo(() => ({
    algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: isDark ? '#6ea8ff' : '#1668dc',
      colorInfo: isDark ? '#6ea8ff' : '#1668dc',
      colorSuccess: '#0f9f6e',
      colorWarning: '#d9901a',
      colorError: '#d64545',
      borderRadius: 10,
      borderRadiusLG: 12,
      colorText: isDark ? '#e8eef6' : '#17283a',
      colorTextSecondary: isDark ? '#98a7b8' : '#5f7082',
      colorBorder: isDark ? '#2a3441' : '#d8e0e8',
      colorBgLayout: 'transparent',
      colorBgContainer: isDark ? '#161d26' : '#ffffff',
      colorBgElevated: isDark ? '#161d26' : '#ffffff',
      colorFillSecondary: isDark ? 'rgba(255, 255, 255, 0.04)' : '#f3f6f9',
      colorFillTertiary: isDark ? 'rgba(255, 255, 255, 0.06)' : '#eef2f6',
      boxShadowTertiary: '0 1px 2px rgba(15, 23, 42, 0.06)',
      fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif',
    },
    components: {
      Button: {
        controlHeight: 38,
        borderRadius: 10,
        fontWeight: 600,
        primaryShadow: 'none',
        defaultShadow: 'none',
      },
      Input: {
        controlHeight: 38,
        borderRadius: 10,
        activeShadow: 'none',
      },
      Select: {
        controlHeight: 38,
        borderRadius: 10,
        optionSelectedBg: isDark ? 'rgba(110, 168, 255, 0.14)' : 'rgba(22, 104, 220, 0.08)',
      },
      Card: {
        borderRadiusLG: 12,
      },
      Table: {
        headerBg: isDark ? '#1b2430' : '#f7f9fb',
        headerColor: isDark ? '#98a7b8' : '#5f7082',
        borderColor: isDark ? '#2a3441' : '#e3e9ef',
        rowHoverBg: isDark ? 'rgba(110, 168, 255, 0.08)' : 'rgba(22, 104, 220, 0.04)',
      },
      Modal: {
        borderRadiusLG: 14,
      },
      Segmented: {
        trackBg: isDark ? '#161d26' : '#f3f6f9',
        itemSelectedBg: isDark ? '#202a36' : '#ffffff',
        itemColor: isDark ? '#98a7b8' : '#5f7082',
        itemSelectedColor: isDark ? '#e8eef6' : '#17283a',
      },
    },
  }), [isDark]);

  return (
    <ConfigProvider
      locale={locale === 'zh' ? zhCN : enUS}
      theme={themeConfig}
    >
      <BrowserRouter basename={APP_BASENAME || undefined}>
        <Routes>
          <Route path="/login" element={<RouteSuspense fullscreen><Login /></RouteSuspense>} />
          <Route path="/" element={<PrivateRoute><MainLayout onPreloadRoutes={preloadAdminRoutes} /></PrivateRoute>}>
            <Route index element={<Navigate to="/dashboard" />} />
            <Route path="dashboard" element={<RouteSuspense><Dashboard /></RouteSuspense>} />
            <Route path="publish" element={<RouteSuspense><Publish /></RouteSuspense>} />
            <Route path="artifacts" element={<RouteSuspense><Artifacts /></RouteSuspense>} />
            <Route path="settings" element={<RouteSuspense><Settings /></RouteSuspense>} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}
