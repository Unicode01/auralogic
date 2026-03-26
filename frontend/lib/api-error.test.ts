import { resolveApiErrorMessage, resolvePluginOperationErrorMessage } from '@/lib/api-error'
import { zhTranslations } from '@/lib/i18n'

describe('resolveApiErrorMessage', () => {
  it('translates password policy messages for generic pages', () => {
    expect(
      resolveApiErrorMessage(
        new Error('Password must contain at least one uppercase letter'),
        zhTranslations,
        zhTranslations.common.failed
      )
    ).toBe(zhTranslations.auth.passwordNeedUppercase)

    expect(
      resolveApiErrorMessage(
        new Error('Password must be at least 10 characters'),
        zhTranslations,
        zhTranslations.common.failed
      )
    ).toBe((zhTranslations.auth.passwordTooShort as string).replace('{n}', '10'))
  })

  it('translates password policy biz errors from shared dictionary', () => {
    expect(
      resolveApiErrorMessage(
        {
          message: 'validation failed',
          data: {
            error_key: 'password.needUppercase',
          },
          errorKey: 'password.needUppercase',
        },
        zhTranslations,
        zhTranslations.common.failed
      )
    ).toBe(zhTranslations.auth.passwordNeedUppercase)
  })

  it('translates order biz errors from shared dictionary', () => {
    expect(
      resolveApiErrorMessage(
        {
          message: 'invalid request',
          data: {
            error_key: 'order.updatePriceStatusInvalid',
            params: {
              status: 'pending',
            },
          },
          errorKey: 'order.updatePriceStatusInvalid',
          errorParams: {
            status: 'pending',
          },
        },
        zhTranslations,
        zhTranslations.common.failed
      )
    ).toBe('只有待付款订单可以修改价格（当前状态：pending）')
  })

  it('formats package manifest schema errors as readable multiline text', () => {
    expect(
      resolveApiErrorMessage(
        {
          message: 'Invalid package manifest at webhooks[0].secret_key: is required when auth_mode is "header"',
          data: {
            error_key: 'plugin.admin.http_400.invalid_package_manifest_schema',
            params: {
              path: 'webhooks[0].secret_key',
              reason: 'is required when auth_mode is "header"',
            },
          },
          errorKey: 'plugin.admin.http_400.invalid_package_manifest_schema',
          errorParams: {
            path: 'webhooks[0].secret_key',
            reason: 'is required when auth_mode is "header"',
          },
        },
        zhTranslations,
        zhTranslations.common.failed
      )
    ).toBe(
      [
        zhTranslations.admin.packageManifestSchemaInvalid,
        `${zhTranslations.admin.packageManifestField}: webhooks[0].secret_key`,
        `${zhTranslations.admin.packageManifestReason}: is required when auth_mode is "header"`,
        zhTranslations.admin.packageManifestFixHint,
      ].join('\n')
    )
  })

  it('formats plugin execution errors with reason, details and task id', () => {
    expect(
      resolvePluginOperationErrorMessage(
        {
          message: 'plugin execute failed',
          data: {
            error_key: 'plugin.admin.http_400.plugin_execute_failed',
            params: {
              cause: 'dial tcp 127.0.0.1:50051: connectex: connection refused',
              details: 'worker exited before response',
            },
            task_id: 'pex_task_001',
          },
          errorKey: 'plugin.admin.http_400.plugin_execute_failed',
          errorParams: {
            cause: 'dial tcp 127.0.0.1:50051: connectex: connection refused',
            details: 'worker exited before response',
          },
        },
        zhTranslations,
        zhTranslations.admin.operationFailed
      )
    ).toBe(
      [
        '插件执行失败',
        '原因: dial tcp 127.0.0.1:50051: connectex: connection refused',
        '详情: worker exited before response',
        '任务 ID: pex_task_001',
      ].join('\n')
    )
  })

  it('uses details as reason when plugin payload does not provide cause', () => {
    expect(
      resolvePluginOperationErrorMessage(
        {
          success: false,
          status: 400,
          message: 'package uploaded but activate failed',
          error_key: 'plugin.admin.http_400.package_uploaded_but_activate_failed',
          error_params: {
            details: 'plugin lifecycle start failed',
          },
          details: 'plugin lifecycle start failed',
        },
        zhTranslations,
        zhTranslations.admin.pluginUploadActivateFailed
      )
    ).toBe(['插件包上传成功，但激活失败', '原因: plugin lifecycle start failed'].join('\n'))
  })
})
