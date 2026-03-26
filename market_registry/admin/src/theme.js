import React from 'react';

const THEME_STORAGE_KEY = 'market_registry.admin.theme';

function normalizeTheme(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'dark') {
    return 'dark';
  }
  return 'light';
}

function getPreferredTheme() {
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return 'light';
}

export function getStoredTheme() {
  try {
    const value = localStorage.getItem(THEME_STORAGE_KEY);
    if (value) {
      return normalizeTheme(value);
    }
  } catch (error) {
  }
  return getPreferredTheme();
}

const ThemeContext = React.createContext({
  themeMode: 'light',
  setThemeMode: () => {},
});

export function ThemeProvider({ children }) {
  const [themeMode, setThemeModeState] = React.useState(() => getStoredTheme());

  const setThemeMode = React.useCallback((nextTheme) => {
    const normalized = normalizeTheme(nextTheme);
    setThemeModeState(normalized);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, normalized);
    } catch (error) {
    }
  }, []);

  React.useEffect(() => {
    if (typeof document === 'undefined') {
      return;
    }
    document.documentElement.dataset.theme = themeMode;
    document.documentElement.style.colorScheme = themeMode;
  }, [themeMode]);

  const value = React.useMemo(() => ({
    themeMode,
    setThemeMode,
  }), [themeMode, setThemeMode]);

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useThemeMode() {
  return React.useContext(ThemeContext);
}
