import { useEffect } from 'react'

export function usePageTitle(title: string) {
  useEffect(() => {
    let appName = 'AuraLogic'
    try {
      appName =
        (window as any).__APP_NAME__ || localStorage.getItem('auralogic_app_name') || 'AuraLogic'
    } catch {
      appName = 'AuraLogic'
    }
    document.title = title ? `${title} - ${appName}` : appName
  }, [title])
}
