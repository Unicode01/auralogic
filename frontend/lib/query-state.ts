'use client'

type SearchParamsLike = {
  toString(): string
}

export function normalizeQueryString(value?: string | null): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function normalizePositivePageQuery(value?: string | null, fallback = 1): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    return fallback
  }
  return parsed
}

export function buildUpdatedQueryString(
  current: SearchParamsLike | URLSearchParams | undefined,
  updates: Record<string, string | number | null | undefined>,
  defaults: Record<string, string | number> = {}
): string {
  const params = new URLSearchParams(current?.toString() || '')

  Object.entries(updates).forEach(([key, value]) => {
    const defaultValue = defaults[key]
    const shouldDelete =
      value === undefined ||
      value === null ||
      value === '' ||
      (defaultValue !== undefined && String(value) === String(defaultValue))

    if (shouldDelete) {
      params.delete(key)
      return
    }

    params.set(key, String(value))
  })

  return params.toString()
}
