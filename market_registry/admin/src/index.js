import React from 'react';
import ReactDOM from 'react-dom/client';
import 'antd/dist/reset.css';
import App from './App';
import { I18nProvider } from './i18n';
import { NavigationGuardProvider } from './navigationGuard';
import { ThemeProvider } from './theme';
import './styles.css';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <ThemeProvider>
    <NavigationGuardProvider>
      <I18nProvider>
        <App />
      </I18nProvider>
    </NavigationGuardProvider>
  </ThemeProvider>
);
