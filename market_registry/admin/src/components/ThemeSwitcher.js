import React from 'react';
import { Segmented } from 'antd';
import { BulbOutlined, MoonOutlined } from '@ant-design/icons';
import { useI18n } from '../i18n';
import { useThemeMode } from '../theme';

export default function ThemeSwitcher({ size = 'middle' }) {
  const { t } = useI18n();
  const { themeMode, setThemeMode } = useThemeMode();

  return (
    <Segmented
      className="market-segmented market-theme-switcher"
      size={size}
      value={themeMode}
      options={[
        {
          label: (
            <span className="market-switcher-option">
              <BulbOutlined />
              <span>{t('浅色')}</span>
            </span>
          ),
          value: 'light',
        },
        {
          label: (
            <span className="market-switcher-option">
              <MoonOutlined />
              <span>{t('暗色')}</span>
            </span>
          ),
          value: 'dark',
        },
      ]}
      onChange={setThemeMode}
    />
  );
}
