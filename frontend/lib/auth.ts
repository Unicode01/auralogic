import { normalizeAuthUser } from '@/lib/auth-user'

const TOKEN_KEY = 'auth_token'
const USER_KEY = 'user_info'
export const AUTH_TOKEN_COOKIE_NAME = 'auralogic_auth_token'
export const AUTH_SESSION_HINT_COOKIE_NAME = 'auralogic_session'
export const AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS = 60 * 60 * 24 * 30

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const prefix = `${name}=`
  for (const part of document.cookie.split(';')) {
    const normalized = part.trim()
    if (!normalized.startsWith(prefix)) continue
    const value = normalized.slice(prefix.length)
    return value ? decodeURIComponent(value) : ''
  }
  return null
}

function writeCookie(
  name: string,
  value: string,
  options?: {
    maxAge?: number
  }
): void {
  if (typeof document === 'undefined') return
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  const maxAge =
    typeof options?.maxAge === 'number' ? `; Max-Age=${Math.max(0, options.maxAge)}` : ''
  document.cookie = `${name}=${encodeURIComponent(value)}; Path=/${maxAge}; SameSite=Lax${secure}`
}

function clearCookie(name: string): void {
  if (typeof document === 'undefined') return
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  document.cookie = `${name}=; Path=/; Max-Age=0; SameSite=Lax${secure}`
}

function setWindowSessionHint(nextValue: boolean): void {
  if (!isBrowser()) return
  ;(window as any).__AURALOGIC_HAS_SESSION__ = nextValue
}

export function markSessionActive(maxAge: number = AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS): void {
  if (!isBrowser()) return
  writeCookie(AUTH_SESSION_HINT_COOKIE_NAME, '1', { maxAge })
  setWindowSessionHint(true)
}

export function clearSessionHint(): void {
  if (!isBrowser()) return
  clearCookie(AUTH_SESSION_HINT_COOKIE_NAME)
  setWindowSessionHint(false)
}

export function hasSessionHint(): boolean {
  if (!isBrowser()) return false
  return (
    readCookie(AUTH_SESSION_HINT_COOKIE_NAME) === '1' || (window as any).__AURALOGIC_HAS_SESSION__ === true
  )
}

export function clearLegacyToken(): void {
  if (!isBrowser()) return
  localStorage.removeItem(TOKEN_KEY)
}

export function getToken(): string | null {
  if (!isBrowser()) return null
  const token = localStorage.getItem(TOKEN_KEY)
  return token ? token.trim() || null : null
}

export function setToken(token: string): void {
  if (!token) return
  markSessionActive()
}

export function clearToken(): void {
  clearLegacyToken()
  localStorage.removeItem(USER_KEY)
  clearSessionHint()
}

export function isAuthenticated(): boolean {
  return hasSessionHint() || !!getToken()
}

export function setUser(user: any): void {
  localStorage.setItem(USER_KEY, JSON.stringify(normalizeAuthUser(user)))
  markSessionActive()
}

export function getUser(): any {
  if (typeof window === 'undefined') return null
  const user = localStorage.getItem(USER_KEY)
  return user ? normalizeAuthUser(JSON.parse(user)) : null
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

