'use client'

import { getUser } from '@/lib/auth'

/**
 * 权限检查 Hook
 */
export function usePermission() {
  const user = getUser()

  /**
   * 检查用户是否有指定权限
   * @param permission 权限标识
   * @returns 是否有权限
   */
  const hasPermission = (permission: string): boolean => {
    if (!user) return false
    
    // 超级管理员拥有所有权限（除了特殊权限）
    if (user.role === 'super_admin') {
      return true
    }
    
    // 检查用户是否有该权限
    return user.permissions?.includes(permission) || false
  }

  /**
   * 检查用户是否有任意一个权限
   * @param permissions 权限数组
   * @returns 是否有权限
   */
  const hasAnyPermission = (permissions: string[]): boolean => {
    return permissions.some((permission) => hasPermission(permission))
  }

  /**
   * 检查用户是否有所有权限
   * @param permissions 权限数组
   * @returns 是否有权限
   */
  const hasAllPermissions = (permissions: string[]): boolean => {
    return permissions.every((permission) => hasPermission(permission))
  }

  /**
   * 检查用户是否是管理员
   */
  const isAdmin = (): boolean => {
    return user?.role === 'admin' || user?.role === 'super_admin'
  }

  /**
   * 检查用户是否是超级管理员
   */
  const isSuperAdmin = (): boolean => {
    return user?.role === 'super_admin'
  }

  return {
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    isAdmin,
    isSuperAdmin,
    user,
  }
}

