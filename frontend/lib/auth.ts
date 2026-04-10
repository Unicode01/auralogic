import { normalizeAuthUser } from '@/lib/auth-user'

const TOKEN_KEY = 'auth_token'
const USER_KEY = 'user_info'
export const AUTH_TOKEN_COOKIE_NAME = 'auralogic_auth_token'
const AUTH_TOKEN_COOKIE_MAX_AGE_SECONDS = 60 * 60 * 24 * 30

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

function writeTokenCookie(token: string): void {
  if (typeof document === 'undefined') return
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  document.cookie =
    `${AUTH_TOKEN_COOKIE_NAME}=${encodeURIComponent(token)}; Path=/; Max-Age=${AUTH_TOKEN_COOKIE_MAX_AGE_SECONDS}; SameSite=Lax${secure}`
}

function clearTokenCookie(): void {
  if (typeof document === 'undefined') return
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  document.cookie = `${AUTH_TOKEN_COOKIE_NAME}=; Path=/; Max-Age=0; SameSite=Lax${secure}`
}

function syncTokenCookie(token: string | null): void {
  if (typeof document === 'undefined') return
  if (!token) {
    if (readCookie(AUTH_TOKEN_COOKIE_NAME) !== null) {
      clearTokenCookie()
    }
    return
  }
  if (readCookie(AUTH_TOKEN_COOKIE_NAME) !== token) {
    writeTokenCookie(token)
  }
}

export function getToken(): string | null {
  if (!isBrowser()) return null
  const token = localStorage.getItem(TOKEN_KEY)
  syncTokenCookie(token)
  return token
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
  syncTokenCookie(token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  syncTokenCookie(null)
}

export function isAuthenticated(): boolean {
  return !!getToken()
}

export function setUser(user: any): void {
  localStorage.setItem(USER_KEY, JSON.stringify(normalizeAuthUser(user)))
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

