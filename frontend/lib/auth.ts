const TOKEN_KEY = 'auth_token'
const USER_KEY = 'user_info'

export function getToken(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export function isAuthenticated(): boolean {
  return !!getToken()
}

export function setUser(user: any): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function getUser(): any {
  if (typeof window === 'undefined') return null
  const user = localStorage.getItem(USER_KEY)
  return user ? JSON.parse(user) : null
}

export function isAdmin(): boolean {
  const user = getUser()
  return user?.role === 'admin' || user?.role === 'super_admin'
}

export function isSuperAdmin(): boolean {
  const user = getUser()
  return user?.role === 'super_admin'
}

export function hasPermission(permission: string): boolean {
  const user = getUser()
  return user?.permissions?.includes(permission) || false
}

