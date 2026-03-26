import React from 'react';

const NavigationGuardContext = React.createContext({
  confirmNavigation: () => true,
  setGuard: () => {},
});

export function NavigationGuardProvider({ children }) {
  const [guard, setGuard] = React.useState(null);

  const confirmNavigation = React.useCallback((overrideMessage = '') => {
    if (!guard?.enabled) {
      return true;
    }
    const message = String(overrideMessage || guard.message || '').trim();
    if (!message || typeof window === 'undefined') {
      return true;
    }
    return window.confirm(message);
  }, [guard]);

  const value = React.useMemo(() => ({
    confirmNavigation,
    setGuard,
  }), [confirmNavigation]);

  return (
    <NavigationGuardContext.Provider value={value}>
      {children}
    </NavigationGuardContext.Provider>
  );
}

export function useNavigationGuard() {
  return React.useContext(NavigationGuardContext);
}

export function usePageNavigationGuard(enabled, message) {
  const { setGuard } = useNavigationGuard();
  const guardIDRef = React.useRef(Symbol('navigation-guard'));

  React.useEffect(() => {
    const guardID = guardIDRef.current;
    if (!enabled) {
      setGuard((current) => (current?.id === guardID ? null : current));
      return undefined;
    }
    const nextGuard = {
      id: guardID,
      enabled: true,
      message,
    };
    setGuard(nextGuard);
    return () => {
      setGuard((current) => (current?.id === guardID ? null : current));
    };
  }, [enabled, message, setGuard]);
}
