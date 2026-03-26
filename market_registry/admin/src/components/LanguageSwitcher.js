import React from 'react';
import { Segmented } from 'antd';
import { useI18n } from '../i18n';

export default function LanguageSwitcher({ size = 'middle' }) {
  const { locale, setLocale } = useI18n();

  return (
    <Segmented
      className="market-segmented market-language-switcher"
      size={size}
      value={locale}
      options={[
        { label: 'EN', value: 'en' },
        { label: '中文', value: 'zh' },
      ]}
      onChange={setLocale}
    />
  );
}
