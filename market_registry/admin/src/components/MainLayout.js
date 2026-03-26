import React from 'react';
import { Button } from 'antd';
import {
  AppstoreOutlined,
  CloudUploadOutlined,
  DashboardOutlined,
  LogoutOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import LanguageSwitcher from './LanguageSwitcher';
import ThemeSwitcher from './ThemeSwitcher';
import { useI18n } from '../i18n';
import { useNavigationGuard } from '../navigationGuard';

function resolveRouteCopy(pathname, routeCopy) {
  if (routeCopy[pathname]) {
    return routeCopy[pathname];
  }
  if (pathname.startsWith('/artifacts')) {
    return routeCopy['/artifacts'];
  }
  if (pathname.startsWith('/publish')) {
    return routeCopy['/publish'];
  }
  if (pathname.startsWith('/settings')) {
    return routeCopy['/settings'];
  }
  return routeCopy['/dashboard'];
}

export default function MainLayout({ onPreloadRoutes }) {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useI18n();
  const { confirmNavigation } = useNavigationGuard();
  const currentUser = React.useMemo(() => {
    try {
      const value = localStorage.getItem('user');
      return value ? JSON.parse(value) : null;
    } catch {
      return null;
    }
  }, []);

  const menuItems = React.useMemo(() => ([
    {
      key: '/dashboard',
      icon: <DashboardOutlined />,
      label: t('仪表盘'),
      description: t('看快照、流量和热点'),
    },
    {
      key: '/artifacts',
      icon: <AppstoreOutlined />,
      label: t('制品管理'),
      description: t('看版本、来源和同步状态'),
    },
    {
      key: '/publish',
      icon: <CloudUploadOutlined />,
      label: t('发布与同步'),
      description: t('发布本地包或同步 Release'),
    },
    {
      key: '/settings',
      icon: <SettingOutlined />,
      label: t('系统设置'),
      description: t('配置制品存储与默认策略'),
    },
  ]), [t]);

  const routeCopy = React.useMemo(() => ({
    '/dashboard': {
      title: t('市场源控制台'),
      description: t('快照、制品和同步入口。'),
    },
    '/artifacts': {
      title: t('制品治理'),
      description: t('版本、来源与回源检查。'),
    },
    '/publish': {
      title: t('发布工作台'),
      description: t('本地发布与远端同步。'),
    },
    '/settings': {
      title: t('系统设置'),
      description: t('配置可选制品存储与默认落点。'),
    },
  }), [t]);

  const shellCopy = resolveRouteCopy(location.pathname, routeCopy);
  const currentRoleLabel = currentUser?.role
    ? (currentUser.role === 'admin' ? t('管理员') : currentUser.role)
    : t('管理员');

  React.useEffect(() => {
    if (typeof onPreloadRoutes !== 'function') {
      return undefined;
    }
    if (typeof window !== 'undefined' && typeof window.requestIdleCallback === 'function') {
      const callbackId = window.requestIdleCallback(() => onPreloadRoutes(), { timeout: 1200 });
      return () => window.cancelIdleCallback(callbackId);
    }
    const timer = window.setTimeout(() => onPreloadRoutes(), 120);
    return () => window.clearTimeout(timer);
  }, [onPreloadRoutes]);

  const handleMenuNavigate = React.useCallback((target) => {
    if (location.pathname === target || location.pathname.startsWith(`${target}/`)) {
      return;
    }
    if (!confirmNavigation()) {
      return;
    }
    navigate(target);
  }, [confirmNavigation, location.pathname, navigate]);

  const handleLogout = () => {
    if (!confirmNavigation()) {
      return;
    }
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  return (
    <div className="market-shell">
      <aside className="market-sidebar">
        <div className="market-sidebar-inner">
          <div className="market-brand">
            <div className="market-brand-mark">M</div>
            <div className="market-brand-copy">
              <div className="market-brand-eyebrow">AuraLogic</div>
              <h1 className="market-brand-title">Market Registry</h1>
              <p className="market-brand-description">
                {t('市场源后台')}
              </p>
            </div>
          </div>

          <div className="market-nav">
            {menuItems.map((item) => {
              const active = location.pathname === item.key || location.pathname.startsWith(`${item.key}/`);
              return (
                <button
                  key={item.key}
                  type="button"
                  className={`market-nav-button${active ? ' is-active' : ''}`}
                  onMouseEnter={() => onPreloadRoutes?.()}
                  onFocus={() => onPreloadRoutes?.()}
                  onClick={() => handleMenuNavigate(item.key)}
                >
                  <span className="market-nav-icon">{item.icon}</span>
                  <span className="market-nav-copy">
                    <span className="market-nav-label">{item.label}</span>
                    <span className="market-nav-description">{item.description}</span>
                  </span>
                </button>
              );
            })}
          </div>

          <div className="market-sidebar-footer">
            <p className="market-sidebar-footer-title">{t('当前环境')}</p>
            <p className="market-sidebar-footer-copy">
              {t('嵌入式后台预览')}
            </p>
            <p className="market-sidebar-footer-copy">
              {t('嵌入式管理界面')} · {t('标准存储')} · {t('Release 同步')}
            </p>
          </div>
        </div>
      </aside>

      <div className="market-main">
        <header className="market-topbar">
          <div className="market-topbar-copy">
            <h2 className="market-topbar-title">{shellCopy.title}</h2>
            <p className="market-topbar-description">{shellCopy.description}</p>
          </div>
          <div className="market-topbar-actions">
            <div className="market-switcher-cluster">
              <ThemeSwitcher size="small" />
              <LanguageSwitcher size="small" />
            </div>
            <div className="market-user-chip">
              <span>{currentUser?.username || 'admin'}</span>
              <span className="market-user-chip-meta">{currentRoleLabel}</span>
            </div>
            <Button icon={<LogoutOutlined />} onClick={handleLogout}>
              {t('退出')}
            </Button>
          </div>
        </header>

        <main className="market-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
