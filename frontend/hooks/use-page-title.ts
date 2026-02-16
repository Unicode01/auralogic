import { useEffect } from 'react'

export function usePageTitle(title: string) {
  useEffect(() => {
    const appName = (window as any).__APP_NAME__
      || localStorage.getItem('auralogic_app_name')
      || 'AuraLogic'
    document.title = title ? `${title} - ${appName}` : appName
  }, [title])
}
