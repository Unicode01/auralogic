import { extractApiErrorInfo, type ApiErrorInfo } from './api'
import { type Translations, translateBizError } from './i18n'

type ResolveApiErrorMessageOptions = {
  useBizError?: boolean
  messageMap?: Record<string, string>
  transform?: (context: {
    info: ApiErrorInfo
    message: string
    errorKey?: string
    errorParams?: Record<string, any>
    fallback: string
    t: Translations
  }) => string | undefined
}

type ErrorRecord = Record<string, any>

const authErrorMessageKeyMap: Record<string, keyof Translations['auth']> = {
  'Invalid email or password': 'invalidEmailOrPassword',
  'User account has been disabled': 'accountDisabled',
  'Password login is disabled, please use quick login or OAuth login': 'passwordLoginDisabled',
  'Please verify your email before logging in': 'emailNotVerified',
  'Captcha is required': 'captchaRequired',
  'Captcha verification failed': 'captchaFailed',
  'Email already in use': 'emailAlreadyInUse',
  'Phone number already in use': 'phoneAlreadyInUse',
  'Registration is disabled': 'registrationDisabled',
  'Invalid request parameters': 'invalidParams',
  'Password must contain at least one uppercase letter': 'passwordNeedUppercase',
  'Password must contain at least one lowercase letter': 'passwordNeedLowercase',
  'Password must contain at least one digit': 'passwordNeedDigit',
  'Password must contain at least one special character': 'passwordNeedSpecial',
  'Verification code expired or invalid': 'codeExpired',
  'Invalid verification code': 'invalidCode',
}

function asRecord(value: unknown): ErrorRecord | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined
  }
  return value as ErrorRecord
}

function readErrorKey(info: ApiErrorInfo): string | undefined {
  const data = asRecord(info.data)
  const key = String(info.errorKey || data?.error_key || data?.errorKey || '').trim()
  return key || undefined
}

function readErrorParams(info: ApiErrorInfo): Record<string, any> | undefined {
  const data = asRecord(info.data)
  return (
    info.errorParams ||
    asRecord(data?.params) ||
    asRecord(data?.error_params) ||
    asRecord(asRecord(data?.data)?.params) ||
    asRecord(asRecord(data?.data)?.error_params)
  )
}

function buildAuthMessageMap(t: Translations): Record<string, string> {
  return Object.entries(authErrorMessageKeyMap).reduce<Record<string, string>>((acc, [message, key]) => {
    const translated = t.auth[key]
    if (typeof translated === 'string' && translated.trim()) {
      acc[message] = translated
    }
    return acc
  }, {})
}

function translatePasswordPolicyMessage(message: string, t: Translations): string | undefined {
  const minLengthMatch = message.match(/^Password must be at least (\d+) characters$/)
  if (minLengthMatch) {
    return (t.auth.passwordTooShort as string).replace('{n}', minLengthMatch[1])
  }

  const staticMap: Record<string, string> = {
    'Password must contain at least one uppercase letter': t.auth.passwordNeedUppercase as string,
    'Password must contain at least one lowercase letter': t.auth.passwordNeedLowercase as string,
    'Password must contain at least one digit': t.auth.passwordNeedDigit as string,
    'Password must contain at least one special character': t.auth.passwordNeedSpecial as string,
  }

  return staticMap[message]
}

function normalizeErrorText(value: unknown): string {
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (typeof value !== 'string') {
    return ''
  }
  const text = value.trim()
  if (!text || text === '<nil>' || text === 'undefined' || text === 'null') {
    return ''
  }
  return text
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function pushErrorRecord(
  records: ErrorRecord[],
  seen: Set<ErrorRecord>,
  value: unknown
): void {
  const record = asRecord(value)
  if (!record || seen.has(record)) return
  seen.add(record)
  records.push(record)
}

function collectErrorRecords(value: unknown): ErrorRecord[] {
  const records: ErrorRecord[] = []
  const seen = new Set<ErrorRecord>()

  pushErrorRecord(records, seen, value)

  const root = asRecord(value)
  const response = asRecord(root?.response)
  const rootData = asRecord(root?.data)
  const responseData = asRecord(response?.data)
  const nestedRootData = asRecord(rootData?.data)
  const nestedResponseData = asRecord(responseData?.data)
  const rootMetadata = asRecord(root?.metadata)
  const rootDataMetadata = asRecord(rootData?.metadata)
  const responseDataMetadata = asRecord(responseData?.metadata)

  pushErrorRecord(records, seen, response)
  pushErrorRecord(records, seen, responseData)
  pushErrorRecord(records, seen, rootData)
  pushErrorRecord(records, seen, nestedRootData)
  pushErrorRecord(records, seen, nestedResponseData)
  pushErrorRecord(records, seen, rootMetadata)
  pushErrorRecord(records, seen, rootDataMetadata)
  pushErrorRecord(records, seen, responseDataMetadata)

  return records
}

function readPathText(records: ErrorRecord[], path: string[]): string {
  for (const record of records) {
    let current: unknown = record
    let valid = true
    for (const segment of path) {
      const currentRecord = asRecord(current)
      if (!currentRecord || !Object.prototype.hasOwnProperty.call(currentRecord, segment)) {
        valid = false
        break
      }
      current = currentRecord[segment]
    }
    if (!valid) continue
    const text = normalizeErrorText(current)
    if (text) return text
  }
  return ''
}

function stripTrailingPluginContext(message: string, suffix: string): string {
  const normalizedMessage = normalizeErrorText(message)
  const normalizedSuffix = normalizeErrorText(suffix)
  if (!normalizedMessage || !normalizedSuffix) {
    return normalizedMessage
  }
  return normalizedMessage
    .replace(new RegExp(`[：:]\\s*${escapeRegExp(normalizedSuffix)}$`), '')
    .trim()
}

function stripTrailingPluginPlaceholder(message: string): string {
  return normalizeErrorText(message)
    .replace(/[：:]\s*\{(?:cause|case|details|reason)\}\s*$/i, '')
    .trim()
}

function normalizePluginErrorTitle(
  title: string,
  cause: string,
  details: string,
  fallback: string,
  t: Translations
): string {
  let normalizedTitle = stripTrailingPluginPlaceholder(title)
  normalizedTitle = stripTrailingPluginContext(normalizedTitle, cause)
  normalizedTitle = stripTrailingPluginContext(normalizedTitle, details)

  if (
    !normalizedTitle ||
    normalizedTitle === cause ||
    normalizedTitle === details ||
    normalizedTitle === 'Request failed'
  ) {
    return fallback || t.admin.pluginBizErrorDefault
  }

  return normalizedTitle
}

function resolvePluginFallbackTitle(fallback: string, t: Translations): string {
  const normalizedFallback = normalizeErrorText(fallback)
  if (
    !normalizedFallback ||
    normalizedFallback === t.common.failed ||
    normalizedFallback === t.admin.operationFailed
  ) {
    return t.admin.pluginBizErrorDefault
  }
  return normalizedFallback
}

export function resolvePluginOperationErrorMessage(
  payload: unknown,
  t: Translations,
  fallback: string
): string {
  const info = extractApiErrorInfo(payload)
  const errorKey = readErrorKey(info)
  const errorParams = readErrorParams(info)
  const fallbackTitle = resolvePluginFallbackTitle(fallback, t)

  const packageManifestMessage = resolvePackageManifestValidationMessage(errorKey, errorParams, t)
  if (packageManifestMessage) {
    return packageManifestMessage
  }

  let title = ''
  if (errorKey) {
    const translated = translateBizError(t, errorKey, errorParams, '')
    if (translated && translated !== errorKey) {
      title = translated.trim()
    }
  }
  if (!title) {
    title = normalizeErrorText(info.message)
  }

  const records = collectErrorRecords(payload)
  const cause =
    normalizeErrorText(errorParams?.cause) ||
    normalizeErrorText(errorParams?.case) ||
    normalizeErrorText(errorParams?.reason) ||
    readPathText(records, ['cause']) ||
    readPathText(records, ['reason']) ||
    normalizeErrorText(errorParams?.details) ||
    readPathText(records, ['details'])
  const details =
    normalizeErrorText(errorParams?.details) ||
    readPathText(records, ['details']) ||
    readPathText(records, ['data', 'details'])
  const taskId =
    readPathText(records, ['task_id']) ||
    readPathText(records, ['taskId']) ||
    readPathText(records, ['plugin_execution_id']) ||
    readPathText(records, ['metadata', 'task_id']) ||
    readPathText(records, ['metadata', 'taskId']) ||
    readPathText(records, ['metadata', 'plugin_execution_id']) ||
    readPathText(records, ['data', 'task_id']) ||
    readPathText(records, ['data', 'taskId']) ||
    readPathText(records, ['data', 'plugin_execution_id'])

  const lines = [normalizePluginErrorTitle(title, cause, details, fallbackTitle, t)]
  if (cause) {
    lines.push(`${t.admin.pluginErrorReason}: ${cause}`)
  }
  if (details && details !== cause) {
    lines.push(`${t.admin.pluginErrorDetails}: ${details}`)
  }
  if (taskId) {
    lines.push(`${t.admin.pluginErrorTaskId}: ${taskId}`)
  }
  return lines.join('\n')
}

export function resolvePackageManifestValidationMessage(
  errorKey: string | undefined,
  errorParams: Record<string, any> | undefined,
  t: Translations
): string | undefined {
  switch (errorKey) {
    case 'plugin.admin.http_400.invalid_package_manifest_json': {
      const cause = normalizeErrorText(errorParams?.cause)
      const lines = [t.admin.packageManifestJsonInvalid]
      if (cause) {
        lines.push(`${t.admin.packageManifestParseError}: ${cause}`)
      }
      lines.push(t.admin.packageManifestFixHint)
      return lines.join('\n')
    }
    case 'plugin.admin.http_400.invalid_package_manifest_schema': {
      const path = normalizeErrorText(errorParams?.path)
      const reason = normalizeErrorText(errorParams?.reason)
      const lines = [t.admin.packageManifestSchemaInvalid]
      if (path) {
        lines.push(`${t.admin.packageManifestField}: ${path}`)
      }
      if (reason) {
        lines.push(`${t.admin.packageManifestReason}: ${reason}`)
      }
      lines.push(t.admin.packageManifestFixHint)
      return lines.join('\n')
    }
    default:
      return undefined
  }
}

export function resolveApiErrorMessage(
  error: unknown,
  t: Translations,
  fallback: string,
  options: ResolveApiErrorMessageOptions = {}
): string {
  const info = extractApiErrorInfo(error)
  const message = String(info.message || '').trim()
  const errorKey = readErrorKey(info)
  const errorParams = readErrorParams(info)
  const normalizedFallback = String(fallback || '').trim() || t.common.failed

  const packageManifestMessage = resolvePackageManifestValidationMessage(errorKey, errorParams, t)
  if (packageManifestMessage) {
    return packageManifestMessage
  }

  if (options.useBizError !== false && errorKey) {
    const translated = translateBizError(t, errorKey, errorParams, '')
    if (translated && translated !== errorKey) {
      return translated
    }
  }

  if (message && options.messageMap?.[message]) {
    return options.messageMap[message]
  }

  const passwordPolicyMessage = translatePasswordPolicyMessage(message, t)
  if (passwordPolicyMessage) {
    return passwordPolicyMessage
  }

  const transformed = options.transform?.({
    info,
    message,
    errorKey,
    errorParams,
    fallback: normalizedFallback,
    t,
  })
  if (typeof transformed === 'string' && transformed.trim()) {
    return transformed.trim()
  }

  return message || normalizedFallback
}

export function resolveAuthApiErrorMessage(
  error: unknown,
  t: Translations,
  fallback: string
): string {
  return resolveApiErrorMessage(error, t, fallback, {
    messageMap: buildAuthMessageMap(t),
    transform: ({ info, message, t: translations }) => {
      if (info.code === 42902) {
        return translations.auth.cooldownWait
      }

      if (message === 'Reset token expired or invalid') {
        return translations.auth.resetTokenExpired
      }

      return undefined
    },
  })
}
