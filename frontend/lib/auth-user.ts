type AuthUserLike = Record<string, any> | null | undefined

export function normalizeAuthUser<T extends AuthUserLike>(user: T): T {
  if (!user || typeof user !== 'object' || Array.isArray(user)) {
    return user
  }

  const normalizedId = user.id ?? user.user_id
  const normalizedCreatedAt = user.createdAt ?? user.created_at
  const normalizedIsActive = user.isActive ?? user.is_active

  return {
    ...user,
    ...(normalizedId !== undefined ? { id: normalizedId, user_id: normalizedId } : {}),
    ...(normalizedCreatedAt !== undefined
      ? { createdAt: normalizedCreatedAt, created_at: normalizedCreatedAt }
      : {}),
    ...(normalizedIsActive !== undefined
      ? { isActive: !!normalizedIsActive, is_active: !!normalizedIsActive }
      : {}),
  } as T
}
