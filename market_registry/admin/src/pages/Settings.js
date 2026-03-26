import React from 'react';
import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  message,
} from 'antd';
import {
  DeleteOutlined,
  PlusOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { api, getAPIErrorMessage } from '../api/client';
import PageHeader from '../components/PageHeader';
import { useI18n } from '../i18n';
import { usePageNavigationGuard } from '../navigationGuard';

const CANONICAL_PROFILE_ID = 'canonical';
const PROFILE_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const FTP_SECURITY_OPTIONS = [
  { value: 'plain', label: 'Plain FTP' },
  { value: 'explicit_tls', label: 'Explicit TLS' },
  { value: 'implicit_tls', label: 'Implicit TLS' },
];

function firstNonEmpty(...values) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
  }
  return '';
}

function defaultProfileName(type) {
  switch (type) {
    case 's3':
      return 'S3 Artifact Storage';
    case 'webdav':
      return 'WebDAV Artifact Storage';
    case 'ftp':
      return 'FTP Artifact Storage';
    default:
      return 'Local Artifact Storage';
  }
}

function createProfile(type) {
  const normalizedType = firstNonEmpty(type, 'local').toLowerCase();
  const unique = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  return {
    id: `${normalizedType}-${unique}`,
    original_id: '',
    name: defaultProfileName(normalizedType),
    type: normalizedType,
    description: '',
    base_dir: '',
    base_url: '',
    s3_endpoint: '',
    s3_region: '',
    s3_bucket: '',
    s3_prefix: '',
    s3_access_key_id: '',
    s3_secret_access_key: '',
    s3_session_token: '',
    has_s3_secret_access_key: false,
    has_s3_session_token: false,
    clear_s3_secret_access_key: false,
    clear_s3_session_token: false,
    s3_use_path_style: false,
    webdav_endpoint: '',
    webdav_username: '',
    webdav_password: '',
    has_webdav_password: false,
    clear_webdav_password: false,
    webdav_skip_verify: false,
    ftp_address: '',
    ftp_username: '',
    ftp_password: '',
    has_ftp_password: false,
    clear_ftp_password: false,
    ftp_root_dir: '',
    ftp_security: 'plain',
    ftp_skip_verify: false,
  };
}

function normalizeSettingsDocument(value) {
  return {
    version: Number(value?.version) > 0 ? Number(value.version) : 1,
    artifact_storage: {
      default_profile_id: firstNonEmpty(value?.artifact_storage?.default_profile_id, CANONICAL_PROFILE_ID),
      profiles: Array.isArray(value?.artifact_storage?.profiles)
        ? value.artifact_storage.profiles.map((profile) => ({
          ...profile,
          id: firstNonEmpty(profile?.id),
          original_id: firstNonEmpty(profile?.original_id, profile?.id),
          name: firstNonEmpty(profile?.name),
          type: firstNonEmpty(profile?.type, 'local').toLowerCase(),
          description: firstNonEmpty(profile?.description),
          base_dir: firstNonEmpty(profile?.base_dir),
          base_url: firstNonEmpty(profile?.base_url),
          s3_endpoint: firstNonEmpty(profile?.s3_endpoint),
          s3_region: firstNonEmpty(profile?.s3_region),
          s3_bucket: firstNonEmpty(profile?.s3_bucket),
          s3_prefix: firstNonEmpty(profile?.s3_prefix),
          s3_access_key_id: firstNonEmpty(profile?.s3_access_key_id),
          s3_secret_access_key: firstNonEmpty(profile?.s3_secret_access_key),
          s3_session_token: firstNonEmpty(profile?.s3_session_token),
          has_s3_secret_access_key: Boolean(profile?.has_s3_secret_access_key),
          has_s3_session_token: Boolean(profile?.has_s3_session_token),
          clear_s3_secret_access_key: Boolean(profile?.clear_s3_secret_access_key),
          clear_s3_session_token: Boolean(profile?.clear_s3_session_token),
          s3_use_path_style: Boolean(profile?.s3_use_path_style),
          webdav_endpoint: firstNonEmpty(profile?.webdav_endpoint),
          webdav_username: firstNonEmpty(profile?.webdav_username),
          webdav_password: firstNonEmpty(profile?.webdav_password),
          has_webdav_password: Boolean(profile?.has_webdav_password),
          clear_webdav_password: Boolean(profile?.clear_webdav_password),
          webdav_skip_verify: Boolean(profile?.webdav_skip_verify),
          ftp_address: firstNonEmpty(profile?.ftp_address),
          ftp_username: firstNonEmpty(profile?.ftp_username),
          ftp_password: firstNonEmpty(profile?.ftp_password),
          has_ftp_password: Boolean(profile?.has_ftp_password),
          clear_ftp_password: Boolean(profile?.clear_ftp_password),
          ftp_root_dir: firstNonEmpty(profile?.ftp_root_dir),
          ftp_security: firstNonEmpty(profile?.ftp_security, 'plain').toLowerCase(),
          ftp_skip_verify: Boolean(profile?.ftp_skip_verify),
          builtin: Boolean(profile?.builtin),
          read_only: Boolean(profile?.read_only),
        }))
        : [],
    },
  };
}

function serializeSettingsDocument(value) {
  return JSON.stringify(normalizeSettingsDocument(value));
}

function collectSettingsValidationIssues(value, t) {
  const normalized = normalizeSettingsDocument(value);
  const profiles = Array.isArray(normalized?.artifact_storage?.profiles)
    ? normalized.artifact_storage.profiles.filter((profile) => profile && !profile.builtin)
    : [];
  const issues = [];
  const seen = new Map();
  const duplicateProfileKeys = new Set();

  const addIssue = (message, profileKey = '') => {
    issues.push({
      message,
      profileKey: firstNonEmpty(profileKey),
    });
  };

  for (const profile of profiles) {
    const profileKey = firstNonEmpty(profile.id, profile.original_id);
    const profileLabel = firstNonEmpty(profile.name, profile.id, t('未命名存储'));
    const profileID = firstNonEmpty(profile.id);
    if (!profileID) {
      addIssue(t('存在未填写 ID 的制品存储配置'), profileKey);
      continue;
    }
    if (!PROFILE_ID_PATTERN.test(profileID)) {
      addIssue(t('制品存储 {value} 的 ID 格式不合法', { value: profileLabel }), profileKey);
    }
    const lowered = profileID.toLowerCase();
    if (seen.has(lowered)) {
      duplicateProfileKeys.add(seen.get(lowered));
      duplicateProfileKeys.add(profileKey);
    } else {
      seen.set(lowered, profileKey);
    }

    if (profile.type === 's3') {
      if (!firstNonEmpty(profile.s3_endpoint)) {
        addIssue(t('制品存储 {value} 缺少 S3 Endpoint', { value: profileLabel }), profileKey);
      }
      if (!firstNonEmpty(profile.s3_bucket)) {
        addIssue(t('制品存储 {value} 缺少 Bucket', { value: profileLabel }), profileKey);
      }
      if (!firstNonEmpty(profile.s3_access_key_id)) {
        addIssue(t('制品存储 {value} 缺少 Access Key ID', { value: profileLabel }), profileKey);
      }
      const hasSecret = Boolean(profile.has_s3_secret_access_key && !profile.clear_s3_secret_access_key);
      const hasNewSecret = Boolean(firstNonEmpty(profile.s3_secret_access_key));
      if (!hasSecret && !hasNewSecret) {
        addIssue(t('制品存储 {value} 缺少 Secret Access Key', { value: profileLabel }), profileKey);
      }
    } else if (profile.type === 'webdav') {
      if (!firstNonEmpty(profile.webdav_endpoint)) {
        addIssue(t('制品存储 {value} 缺少 WebDAV Endpoint', { value: profileLabel }), profileKey);
      }
    } else if (profile.type === 'ftp') {
      if (!firstNonEmpty(profile.ftp_address)) {
        addIssue(t('制品存储 {value} 缺少 FTP 地址', { value: profileLabel }), profileKey);
      }
      if (!FTP_SECURITY_OPTIONS.some((item) => item.value === firstNonEmpty(profile.ftp_security, 'plain').toLowerCase())) {
        addIssue(t('制品存储 {value} 的 FTP 安全模式不合法', { value: profileLabel }), profileKey);
      }
    } else if (profile.type === 'local') {
      if (!firstNonEmpty(profile.base_dir)) {
        addIssue(t('制品存储 {value} 缺少本地目录', { value: profileLabel }), profileKey);
      }
    } else {
      addIssue(t('制品存储 {value} 的类型不受支持', { value: profileLabel }), profileKey);
    }
  }

  duplicateProfileKeys.forEach((profileKey) => {
    addIssue(t('制品存储 ID 不能重复'), profileKey);
  });

  const defaultProfileID = firstNonEmpty(normalized?.artifact_storage?.default_profile_id, CANONICAL_PROFILE_ID);
  if (
    defaultProfileID !== CANONICAL_PROFILE_ID &&
    !profiles.some((profile) => profile && profile.id === defaultProfileID)
  ) {
    addIssue(t('默认制品存储不存在，请重新选择'));
  }
  return issues;
}

export default function Settings() {
  const { t } = useI18n();
  const initialSettings = React.useMemo(() => normalizeSettingsDocument({}), []);
  const [settings, setSettings] = React.useState(initialSettings);
  const [savedSnapshot, setSavedSnapshot] = React.useState(() => serializeSettingsDocument(initialSettings));
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  const loadSettings = React.useCallback(async ({ silent = false } = {}) => {
    setLoading(true);
    try {
      const response = await api.getSettings();
      const normalized = normalizeSettingsDocument(response?.data?.data || {});
      setSettings(normalized);
      setSavedSnapshot(serializeSettingsDocument(normalized));
    } catch (error) {
      if (!silent) {
        message.error(getAPIErrorMessage(error, t('加载系统设置失败')));
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const profiles = Array.isArray(settings?.artifact_storage?.profiles) ? settings.artifact_storage.profiles : [];
  const canonicalProfile = profiles.find((profile) => profile?.id === CANONICAL_PROFILE_ID || profile?.builtin) || null;
  const customProfiles = profiles.filter((profile) => profile && profile.id !== CANONICAL_PROFILE_ID && !profile.builtin);
  const defaultProfileID = firstNonEmpty(settings?.artifact_storage?.default_profile_id, CANONICAL_PROFILE_ID);
  const validationIssues = React.useMemo(() => collectSettingsValidationIssues(settings, t), [settings, t]);
  const validationErrors = React.useMemo(
    () => Array.from(new Set(validationIssues.map((item) => item.message))),
    [validationIssues]
  );
  const validationByProfile = React.useMemo(
    () => validationIssues.reduce((accumulator, item) => {
      const profileKey = firstNonEmpty(item?.profileKey);
      if (!profileKey) {
        return accumulator;
      }
      if (!accumulator[profileKey]) {
        accumulator[profileKey] = [];
      }
      if (!accumulator[profileKey].includes(item.message)) {
        accumulator[profileKey].push(item.message);
      }
      return accumulator;
    }, {}),
    [validationIssues]
  );
  const hasUnsavedChanges = React.useMemo(
    () => serializeSettingsDocument(settings) !== savedSnapshot,
    [savedSnapshot, settings]
  );
  usePageNavigationGuard(
    hasUnsavedChanges,
    t('当前页面有未保存修改，离开后将丢失这些更改。是否继续？')
  );
  React.useEffect(() => {
    if (!hasUnsavedChanges) {
      return undefined;
    }
    const handleBeforeUnload = (event) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [hasUnsavedChanges]);
  const profileOptions = profiles
    .filter((profile) => profile?.id)
    .map((profile) => ({
      value: profile.id,
      label: `${firstNonEmpty(profile.name, profile.id)} · ${String(profile.type || 'local').toUpperCase()}${profile.builtin ? ` · ${t('系统内置')}` : ''}`,
    }));

  const updateSettings = (updater) => {
    setSettings((current) => normalizeSettingsDocument(
      typeof updater === 'function' ? updater(normalizeSettingsDocument(current)) : updater
    ));
  };

  const updateProfile = (profileID, patch) => {
    updateSettings((current) => ({
      ...current,
      artifact_storage: (() => {
        let nextProfileID = profileID;
        const nextProfiles = current.artifact_storage.profiles.map((profile) => {
          if (!profile || profile.id !== profileID) {
            return profile;
          }
          const nextProfile = {
            ...profile,
            ...(typeof patch === 'function' ? patch(profile) : patch),
          };
          nextProfileID = firstNonEmpty(nextProfile.id, profileID);
          return nextProfile;
        });
        return {
          ...current.artifact_storage,
          default_profile_id: current.artifact_storage.default_profile_id === profileID
            ? nextProfileID
            : current.artifact_storage.default_profile_id,
          profiles: nextProfiles,
        };
      })(),
    }));
  };

  const removeProfile = (profileID) => {
    updateSettings((current) => {
      const nextProfiles = current.artifact_storage.profiles.filter((profile) => profile && profile.id !== profileID);
      return {
        ...current,
        artifact_storage: {
          ...current.artifact_storage,
          default_profile_id: current.artifact_storage.default_profile_id === profileID
            ? CANONICAL_PROFILE_ID
            : current.artifact_storage.default_profile_id,
          profiles: nextProfiles,
        },
      };
    });
  };

  const addProfile = (type) => {
    const nextProfile = createProfile(type);
    updateSettings((current) => ({
      ...current,
      artifact_storage: {
        ...current.artifact_storage,
        profiles: [...current.artifact_storage.profiles, nextProfile],
      },
    }));
  };

  const handleSave = async () => {
    if (validationErrors.length > 0) {
      message.error(validationErrors[0]);
      return;
    }
    setSaving(true);
    try {
      const response = await api.updateSettings(settings);
      const normalized = normalizeSettingsDocument(response?.data?.data || settings);
      setSettings(normalized);
      setSavedSnapshot(serializeSettingsDocument(normalized));
      message.success(response?.data?.message || t('系统设置已保存'));
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('保存系统设置失败')));
    } finally {
      setSaving(false);
    }
  };

  const handleReload = () => {
    if (!hasUnsavedChanges) {
      void loadSettings();
      return;
    }
    Modal.confirm({
      title: t('放弃未保存更改？'),
      content: t('重新加载会丢弃当前页面中的未保存修改。'),
      okText: t('放弃并重新加载'),
      cancelText: t('继续编辑'),
      onOk: () => loadSettings(),
    });
  };

  if (loading) {
    return (
      <div className="market-page">
        <Card className="market-panel">
          <Spin />
        </Card>
      </div>
    );
  }

  return (
    <div className="market-page">
      <PageHeader
        eyebrow={t('Registry')}
        title={t('系统设置')}
        description={t('集中配置制品存储 profile，并指定上传与同步时的默认落点。')}
        meta={[
          { label: t('默认制品存储'), value: firstNonEmpty(defaultProfileID, CANONICAL_PROFILE_ID) },
          { label: t('自定义存储数'), value: customProfiles.length },
          { label: t('当前状态'), value: hasUnsavedChanges ? t('有未保存更改') : t('已同步') },
        ]}
        actions={(
          <Space>
            {hasUnsavedChanges ? <Tag color="warning">{t('未保存')}</Tag> : <Tag>{t('已保存')}</Tag>}
            <Button onClick={handleReload}>{t('重新加载')}</Button>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSave}
              loading={saving}
              disabled={!hasUnsavedChanges || validationErrors.length > 0}
            >
              {t('保存设置')}
            </Button>
          </Space>
        )}
      />

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={t('manifest、origin、索引等元数据仍保存在 canonical 主存储中。这里配置的是制品 zip 包的可选存储位置。')}
      />
      {validationErrors.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('当前配置还不能保存')}
          description={validationErrors.join('；')}
        />
      )}

      <Card className="market-panel" title={t('默认制品存储')} style={{ marginBottom: 16 }}>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <div className="market-inline-note">{t('未显式选择存储时，发布和同步都会落到这里。')}</div>
          <Select
            value={defaultProfileID}
            options={profileOptions}
            showSearch
            optionFilterProp="label"
            onChange={(value) => updateSettings((current) => ({
              ...current,
              artifact_storage: {
                ...current.artifact_storage,
                default_profile_id: value,
              },
            }))}
          />
        </Space>
      </Card>

      {canonicalProfile && (
        <Card className="market-panel" title={t('Canonical 主存储')} style={{ marginBottom: 16 }}>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Alert
              type="success"
              showIcon
              message={t('该 profile 由运行时配置生成，只读展示，不能在页面内修改。')}
            />
            <div className="market-form-grid market-form-grid-3">
              <Input addonBefore={t('ID')} value={canonicalProfile.id} readOnly />
              <Input addonBefore={t('名称')} value={canonicalProfile.name || canonicalProfile.id} readOnly />
              <Input addonBefore={t('类型')} value={String(canonicalProfile.type || 'local').toUpperCase()} readOnly />
            </div>
            {canonicalProfile.type === 's3' ? (
              <div className="market-form-grid market-form-grid-3">
                <Input addonBefore={t('S3 Endpoint')} value={canonicalProfile.s3_endpoint || '-'} readOnly />
                <Input addonBefore={t('Bucket')} value={canonicalProfile.s3_bucket || '-'} readOnly />
                <Input addonBefore={t('Prefix')} value={canonicalProfile.s3_prefix || '-'} readOnly />
              </div>
            ) : canonicalProfile.type === 'webdav' ? (
              <div className="market-form-grid market-form-grid-3">
                <Input addonBefore={t('WebDAV Endpoint')} value={canonicalProfile.webdav_endpoint || '-'} readOnly />
                <Input addonBefore={t('用户名')} value={canonicalProfile.webdav_username || '-'} readOnly />
                <Input addonBefore={t('Base URL')} value={canonicalProfile.base_url || '-'} readOnly />
              </div>
            ) : canonicalProfile.type === 'ftp' ? (
              <div className="market-form-grid market-form-grid-2">
                <Input addonBefore={t('FTP 地址')} value={canonicalProfile.ftp_address || '-'} readOnly />
                <Input addonBefore={t('Root Dir')} value={canonicalProfile.ftp_root_dir || '/'} readOnly />
                <Input addonBefore={t('FTP 安全模式')} value={canonicalProfile.ftp_security || 'plain'} readOnly />
                <Input addonBefore={t('Base URL')} value={canonicalProfile.base_url || '-'} readOnly />
              </div>
            ) : (
              <div className="market-form-grid market-form-grid-2">
                <Input addonBefore={t('本地目录')} value={canonicalProfile.base_dir || '-'} readOnly />
                <Input addonBefore={t('Base URL')} value={canonicalProfile.base_url || '-'} readOnly />
              </div>
            )}
          </Space>
        </Card>
      )}

      <Card
        className="market-panel"
        title={t('自定义制品存储')}
        extra={(
          <Space>
            <Button icon={<PlusOutlined />} onClick={() => addProfile('local')}>
              {t('新增本地存储')}
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addProfile('s3')}>
              {t('新增 S3 存储')}
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addProfile('webdav')}>
              {t('新增 WebDAV 存储')}
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addProfile('ftp')}>
              {t('新增 FTP 存储')}
            </Button>
          </Space>
        )}
      >
        {customProfiles.length === 0 ? (
          <Empty description={t('暂未配置自定义制品存储')} />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {customProfiles.map((profile) => {
              const profileIssues = validationByProfile[firstNonEmpty(profile.id, profile.original_id)] || [];

              return (
                <Card
                  key={profile.id}
                  size="small"
                  title={firstNonEmpty(profile.name, profile.id)}
                  extra={(
                    <Space size={8} wrap>
                      {defaultProfileID === profile.id ? <Tag color="blue">{t('默认')}</Tag> : null}
                      {profileIssues.length > 0 ? <Tag color="warning">{t('待补全')}</Tag> : null}
                      <Popconfirm
                        title={t('删除制品存储 {value}？', { value: firstNonEmpty(profile.name, profile.id) })}
                        description={defaultProfileID === profile.id
                          ? t('删除后默认制品存储会自动回退到 canonical 主存储。')
                          : t('删除后需要点击保存才会真正生效。')}
                        okText={t('删除')}
                        cancelText={t('取消')}
                        onConfirm={() => removeProfile(profile.id)}
                      >
                        <Button
                          danger
                          type="text"
                          icon={<DeleteOutlined />}
                        >
                          {t('删除')}
                        </Button>
                      </Popconfirm>
                    </Space>
                  )}
                >
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    {profileIssues.length > 0 && (
                      <Alert
                        type="warning"
                        showIcon
                        message={t('该存储配置还不完整')}
                        description={profileIssues.join('；')}
                      />
                    )}
                    <div className="market-form-grid market-form-grid-3">
                      <Input
                        addonBefore={t('ID')}
                        value={profile.id}
                        onChange={(event) => updateProfile(profile.id, { id: event.target.value })}
                      />
                      <Input
                        addonBefore={t('名称')}
                        value={profile.name}
                        onChange={(event) => updateProfile(profile.id, { name: event.target.value })}
                      />
                      <Select
                        value={profile.type}
                        options={[
                          { value: 'local', label: t('本地存储') },
                          { value: 's3', label: t('S3 存储') },
                          { value: 'webdav', label: t('WebDAV 存储') },
                          { value: 'ftp', label: t('FTP 存储') },
                        ]}
                        showSearch
                        optionFilterProp="label"
                        onChange={(value) => updateProfile(profile.id, (current) => ({
                          ...current,
                          type: value,
                          clear_s3_secret_access_key: false,
                          clear_s3_session_token: false,
                          s3_secret_access_key: '',
                          s3_session_token: '',
                          clear_webdav_password: false,
                          webdav_password: '',
                          clear_ftp_password: false,
                          ftp_password: '',
                          ftp_security: value === 'ftp' ? firstNonEmpty(current.ftp_security, 'plain').toLowerCase() : current.ftp_security,
                        }))}
                      />
                    </div>
                    <Input
                      addonBefore={t('说明')}
                      value={profile.description}
                      onChange={(event) => updateProfile(profile.id, { description: event.target.value })}
                    />

                    {profile.type === 's3' ? (
                      <>
                        <div className="market-form-grid market-form-grid-3">
                          <Input
                            addonBefore={t('S3 Endpoint')}
                            value={profile.s3_endpoint}
                            onChange={(event) => updateProfile(profile.id, { s3_endpoint: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Region')}
                            value={profile.s3_region}
                            onChange={(event) => updateProfile(profile.id, { s3_region: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Bucket')}
                            value={profile.s3_bucket}
                            onChange={(event) => updateProfile(profile.id, { s3_bucket: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Prefix')}
                            value={profile.s3_prefix}
                            onChange={(event) => updateProfile(profile.id, { s3_prefix: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Base URL')}
                            value={profile.base_url}
                            onChange={(event) => updateProfile(profile.id, { base_url: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Access Key ID')}
                            value={profile.s3_access_key_id}
                            onChange={(event) => updateProfile(profile.id, { s3_access_key_id: event.target.value })}
                          />
                        </div>
                        <Input.Password
                          addonBefore={t('Secret Access Key')}
                          value={profile.s3_secret_access_key}
                          placeholder={profile.has_s3_secret_access_key ? t('已保存，留空则保持不变') : t('请输入 Secret Access Key')}
                          onChange={(event) => updateProfile(profile.id, {
                            s3_secret_access_key: event.target.value,
                            clear_s3_secret_access_key: false,
                          })}
                        />
                        <div className="market-switch-row">
                          <span>{profile.has_s3_secret_access_key ? t('已保存 Secret Access Key') : t('尚未保存 Secret Access Key')}</span>
                          <Tag>{t('输入新值可替换')}</Tag>
                        </div>
                        <Input.Password
                          addonBefore={t('Session Token')}
                          value={profile.s3_session_token}
                          placeholder={profile.has_s3_session_token ? t('已保存，留空则保持不变') : t('可选；未填写则不使用')}
                          onChange={(event) => updateProfile(profile.id, {
                            s3_session_token: event.target.value,
                            clear_s3_session_token: false,
                          })}
                        />
                        <div className="market-switch-row">
                          <span>{profile.has_s3_session_token ? t('已保存 Session Token') : t('未配置 Session Token')}</span>
                          <Switch
                            checked={Boolean(profile.clear_s3_session_token)}
                            disabled={!profile.has_s3_session_token}
                            onChange={(checked) => updateProfile(profile.id, {
                              clear_s3_session_token: checked,
                              s3_session_token: checked ? '' : profile.s3_session_token,
                            })}
                          />
                        </div>
                        <div className="market-switch-row">
                          <span>{t('强制 Path-Style')}</span>
                          <Switch
                            checked={Boolean(profile.s3_use_path_style)}
                            onChange={(checked) => updateProfile(profile.id, { s3_use_path_style: checked })}
                          />
                        </div>
                      </>
                    ) : profile.type === 'webdav' ? (
                      <>
                        <div className="market-form-grid market-form-grid-3">
                          <Input
                            addonBefore={t('WebDAV Endpoint')}
                            value={profile.webdav_endpoint}
                            onChange={(event) => updateProfile(profile.id, { webdav_endpoint: event.target.value })}
                          />
                          <Input
                            addonBefore={t('用户名')}
                            value={profile.webdav_username}
                            onChange={(event) => updateProfile(profile.id, { webdav_username: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Base URL')}
                            value={profile.base_url}
                            onChange={(event) => updateProfile(profile.id, { base_url: event.target.value })}
                          />
                        </div>
                        <Input.Password
                          addonBefore={t('密码')}
                          value={profile.webdav_password}
                          placeholder={profile.has_webdav_password ? t('已保存，留空则保持不变') : t('可选；未填写则不使用')}
                          onChange={(event) => updateProfile(profile.id, {
                            webdav_password: event.target.value,
                            clear_webdav_password: false,
                          })}
                        />
                        <div className="market-switch-row">
                          <span>{profile.has_webdav_password ? t('已保存 WebDAV 密码') : t('未配置 WebDAV 密码')}</span>
                          <Switch
                            checked={Boolean(profile.clear_webdav_password)}
                            disabled={!profile.has_webdav_password}
                            onChange={(checked) => updateProfile(profile.id, {
                              clear_webdav_password: checked,
                              webdav_password: checked ? '' : profile.webdav_password,
                            })}
                          />
                        </div>
                        <div className="market-switch-row">
                          <span>{t('跳过 TLS 校验')}</span>
                          <Switch
                            checked={Boolean(profile.webdav_skip_verify)}
                            onChange={(checked) => updateProfile(profile.id, { webdav_skip_verify: checked })}
                          />
                        </div>
                      </>
                    ) : profile.type === 'ftp' ? (
                      <>
                        <div className="market-form-grid market-form-grid-3">
                          <Input
                            addonBefore={t('FTP 地址')}
                            value={profile.ftp_address}
                            onChange={(event) => updateProfile(profile.id, { ftp_address: event.target.value })}
                          />
                          <Input
                            addonBefore={t('用户名')}
                            value={profile.ftp_username}
                            onChange={(event) => updateProfile(profile.id, { ftp_username: event.target.value })}
                          />
                          <Input
                            addonBefore={t('Root Dir')}
                            value={profile.ftp_root_dir}
                            onChange={(event) => updateProfile(profile.id, { ftp_root_dir: event.target.value })}
                          />
                          <Select
                            value={firstNonEmpty(profile.ftp_security, 'plain').toLowerCase()}
                            options={FTP_SECURITY_OPTIONS.map((item) => ({
                              value: item.value,
                              label: t(item.label),
                            }))}
                            onChange={(value) => updateProfile(profile.id, { ftp_security: value })}
                          />
                          <Input
                            addonBefore={t('Base URL')}
                            value={profile.base_url}
                            onChange={(event) => updateProfile(profile.id, { base_url: event.target.value })}
                          />
                        </div>
                        <Input.Password
                          addonBefore={t('密码')}
                          value={profile.ftp_password}
                          placeholder={profile.has_ftp_password ? t('已保存，留空则保持不变') : t('可选；未填写则使用匿名登录')}
                          onChange={(event) => updateProfile(profile.id, {
                            ftp_password: event.target.value,
                            clear_ftp_password: false,
                          })}
                        />
                        <div className="market-switch-row">
                          <span>{profile.has_ftp_password ? t('已保存 FTP 密码') : t('未配置 FTP 密码')}</span>
                          <Switch
                            checked={Boolean(profile.clear_ftp_password)}
                            disabled={!profile.has_ftp_password}
                            onChange={(checked) => updateProfile(profile.id, {
                              clear_ftp_password: checked,
                              ftp_password: checked ? '' : profile.ftp_password,
                            })}
                          />
                        </div>
                        <div className="market-switch-row">
                          <span>{t('跳过 TLS 校验')}</span>
                          <Switch
                            checked={Boolean(profile.ftp_skip_verify)}
                            onChange={(checked) => updateProfile(profile.id, { ftp_skip_verify: checked })}
                          />
                        </div>
                      </>
                    ) : (
                      <div className="market-form-grid market-form-grid-2">
                        <Input
                          addonBefore={t('本地目录')}
                          value={profile.base_dir}
                          onChange={(event) => updateProfile(profile.id, { base_dir: event.target.value })}
                        />
                        <Input
                          addonBefore={t('Base URL')}
                          value={profile.base_url}
                          onChange={(event) => updateProfile(profile.id, { base_url: event.target.value })}
                        />
                      </div>
                    )}
                  </Space>
                </Card>
              );
            })}
          </Space>
        )}
      </Card>
      <Card className="market-panel" style={{ marginTop: 16 }}>
        <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
          <div className="market-inline-note">
            {hasUnsavedChanges ? t('当前页面有未保存修改。') : t('当前配置已保存。')}
          </div>
          <Space>
            <Button onClick={handleReload}>{t('重新加载')}</Button>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSave}
              loading={saving}
              disabled={!hasUnsavedChanges || validationErrors.length > 0}
            >
              {t('保存设置')}
            </Button>
          </Space>
        </Space>
      </Card>
    </div>
  );
}
