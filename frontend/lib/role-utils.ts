// 角色相关工具函数

export const roleLabels: Record<string, string> = {
  user: '普通用户',
  admin: '管理员',
  super_admin: '超级管理员',
}

export const roleColors: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  user: 'secondary',
  admin: 'default',
  super_admin: 'destructive',
}

export function getRoleLabel(role: string): string {
  return roleLabels[role] || role
}

export function getRoleColor(role: string) {
  return roleColors[role] || 'secondary'
}

export function isAdmin(role?: string): boolean {
  return role === 'admin' || role === 'super_admin'
}

export function isSuperAdmin(role?: string): boolean {
  return role === 'super_admin'
}

