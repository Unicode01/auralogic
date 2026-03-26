import React, { useState } from 'react';
import { Form, Input, Select, Upload, Button, Card, message, Progress, Descriptions, Space, Alert, Typography, Row, Col } from 'antd';
import { InboxOutlined } from '@ant-design/icons';
import { api, getAPIErrorMessage } from '../api/client';
import JSZip from 'jszip';
import PageHeader from '../components/PageHeader';
import RegistryStatusCard from '../components/RegistryStatusCard';
import { parseGitHubReleaseURL } from '../utils/githubReleaseUrl';
import { useI18n } from '../i18n';

const MAX_ARTIFACT_FILE_SIZE = 100 * 1024 * 1024;
const PUBLISH_FIELD_NAMES = ['kind', 'name', 'version', 'channel'];
const SYNC_FIELD_NAMES = ['sync_source_url', 'sync_kind', 'sync_name', 'sync_version', 'sync_channel', 'sync_owner', 'sync_repo', 'sync_tag', 'sync_asset', 'sync_api_base_url'];
const SYNC_ADVANCED_FIELD_NAMES = [
  'sync_kind',
  'sync_name',
  'sync_version',
  'sync_channel',
  'sync_owner',
  'sync_repo',
  'sync_tag',
  'sync_api_base_url',
  'sync_title',
  'sync_summary',
  'sync_description',
  'sync_release_notes',
];
const SYNC_SOURCE_FIELD_NAMES = ['sync_source_url', 'sync_owner', 'sync_repo', 'sync_tag', 'sync_asset', 'sync_api_base_url'];
const SYNC_PREVIEW_DEPENDENCY_FIELDS = new Set(['sync_source_url', 'sync_owner', 'sync_repo', 'sync_tag', 'sync_api_base_url', 'sync_token']);
const SYNC_INSPECTION_DEPENDENCY_FIELDS = new Set(['sync_source_url', 'sync_kind', 'sync_name', 'sync_version', 'sync_owner', 'sync_repo', 'sync_tag', 'sync_asset', 'sync_api_base_url', 'sync_token']);

function firstNonEmpty(...values) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
  }
  return '';
}

function formatDateTime(value, locale) {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US');
}

function formatBytes(value) {
  const size = Number(value);
  if (!Number.isFinite(size) || size <= 0) {
    return '-';
  }
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function formatRegistryStatus(status, t) {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'healthy') {
    return t('健康');
  }
  if (normalized === 'idle') {
    return t('未初始化');
  }
  if (normalized === 'missing') {
    return t('缺失');
  }
  if (normalized === 'stale') {
    return t('过期');
  }
  if (normalized === 'degraded') {
    return t('异常');
  }
  return status || t('未知');
}

function inferArtifactKind(manifest) {
  if (!manifest || typeof manifest !== 'object') {
    return '';
  }
  if (typeof manifest.kind === 'string' && manifest.kind.trim()) {
    return manifest.kind.trim();
  }
  switch (String(manifest.runtime || '').trim().toLowerCase()) {
    case 'js_worker':
    case 'grpc':
      return 'plugin_package';
    case 'payment_js':
      return 'payment_package';
    default:
      break;
  }
  const resolvedKey = firstNonEmpty(
    manifest.key,
    manifest.target_key,
    manifest.targets?.key,
    manifest.template?.key,
  ).toLowerCase();
  if (resolvedKey === 'invoice') {
    return 'invoice_template';
  }
  if (resolvedKey === 'auth_branding') {
    return 'auth_branding_template';
  }
  if (resolvedKey === 'page_rules') {
    return 'page_rule_pack';
  }
  if (
    (typeof manifest.content_file === 'string' && manifest.content_file.trim()) ||
    (typeof manifest.rules_file === 'string' && manifest.rules_file.trim())
  ) {
    if (typeof manifest.event === 'string' && manifest.event.trim()) {
      return 'email_template';
    }
    if (typeof manifest.engine === 'string' && manifest.engine.trim()) {
      return 'landing_page_template';
    }
  }
  return '';
}

function resolveManifestCoordinates(manifest) {
  return {
    kind: inferArtifactKind(manifest),
    name: firstNonEmpty(manifest?.name),
    version: firstNonEmpty(manifest?.version),
  };
}

function clearFieldErrors(form, fieldNames) {
  form.setFields(fieldNames.map((name) => ({ name, errors: [] })));
}

function coordinateFieldError(messageText, t, prefix = '') {
  const trimmed = firstNonEmpty(messageText);
  let match = trimmed.match(/^(kind|name|version|channel) is required$/);
  if (match) {
    return { name: `${prefix}${match[1]}`, errors: [t('该字段不能为空')] };
  }
  match = trimmed.match(/^(kind|name|version|channel) ".+" does not match manifest \1 ".+"$/);
  if (match) {
    return { name: `${prefix}${match[1]}`, errors: [t('必须与 manifest.json 中的值一致')] };
  }
  match = trimmed.match(/^(kind|name|version|channel) contains forbidden path characters$/);
  if (match) {
    return { name: `${prefix}${match[1]}`, errors: [t('不能包含 /、\\ 或 ..')] };
  }
  match = trimmed.match(/^(kind|name|version|channel) must be a safe identifier$/);
  if (match) {
    return {
      name: `${prefix}${match[1]}`,
      errors: [match[1] === 'version' ? t('仅支持字母、数字、点、下划线、连字符和 +') : t('仅支持字母、数字、点、下划线和连字符')],
    };
  }
  if (/^unsupported artifact kind /.test(trimmed)) {
    return { name: `${prefix}kind`, errors: [t('制品类型不受支持')] };
  }
  return null;
}

function buildPublishFieldErrors(messageText, t) {
  const fieldError = coordinateFieldError(messageText, t);
  return fieldError ? [fieldError] : [];
}

function buildSyncFieldErrors(messageText, t) {
  const trimmed = firstNonEmpty(messageText);
  const coordinateError = coordinateFieldError(trimmed, t, 'sync_');
  if (coordinateError) {
    return [coordinateError];
  }
  if (trimmed === 'github owner is required') {
    return [{ name: 'sync_owner', errors: [t('请输入 owner')] }];
  }
  if (trimmed === 'github repo is required') {
    return [{ name: 'sync_repo', errors: [t('请输入 repo')] }];
  }
  if (trimmed === 'github release tag is required') {
    return [{ name: 'sync_tag', errors: [t('请输入 tag')] }];
  }
  if (trimmed === 'github release asset name is required') {
    return [{ name: 'sync_asset', errors: [t('请输入资产文件名')] }];
  }
  if (trimmed === 'github api base url is invalid') {
    return [{ name: 'sync_api_base_url', errors: [t('请输入合法的 GitHub API Base URL')] }];
  }
  if (trimmed === 'github api base url must use http or https') {
    return [{ name: 'sync_api_base_url', errors: [t('只支持 http 或 https')] }];
  }
  if (trimmed === 'github api base url host is required') {
    return [{ name: 'sync_api_base_url', errors: [t('必须包含主机名')] }];
  }
  if (/^github release asset ".+" was not found$/.test(trimmed)) {
    return [{ name: 'sync_asset', errors: [t('远端 Release 中找不到该资产文件')] }];
  }
  if (trimmed === 'github release asset is empty') {
    return [{ name: 'sync_asset', errors: [t('远端资产为空')] }];
  }
  if (trimmed === 'github release asset exceeds size limit') {
    return [{ name: 'sync_asset', errors: [t('远端资产超过大小限制')] }];
  }
  if (trimmed === 'github release asset size mismatch') {
    return [{ name: 'sync_asset', errors: [t('远端资产大小与 Release 元数据不一致')] }];
  }
  if (/^parse synced artifact manifest failed:/.test(trimmed)) {
    return [{ name: 'sync_asset', errors: [t('远端 zip 无法解析 manifest.json')] }];
  }
  return [];
}

function applyFieldErrors(form, fieldErrors) {
  if (!Array.isArray(fieldErrors) || fieldErrors.length === 0) {
    return false;
  }
  form.setFields(fieldErrors);
  return true;
}

function buildSyncInspectionMismatchFieldErrors(result, t) {
  const changedFields = Array.isArray(result?.changed_fields) ? result.changed_fields : [];
  const fieldNames = new Set(changedFields.filter((item) => ['kind', 'name', 'version'].includes(item)));
  return Array.from(fieldNames).map((name) => ({
    name: `sync_${name}`,
    errors: [t('与远端包内 manifest 不一致')],
  }));
}

function shouldRevealSyncAdvanced(fieldErrors) {
  if (!Array.isArray(fieldErrors) || fieldErrors.length === 0) {
    return false;
  }
  return fieldErrors.some((item) => SYNC_ADVANCED_FIELD_NAMES.includes(String(item?.name || '')));
}

const DESCRIPTION_COLUMNS = { xs: 1, md: 2 };

function buildArtifactStorageAlert(profile, defaultProfileID, t) {
  if (!profile) {
    return {
      type: 'warning',
      message: t('当前选择的制品存储不存在'),
      description: t('请重新选择一个有效的存储目标。'),
    };
  }

  const label = firstNonEmpty(profile.name, profile.id, profile.builtin ? t('Canonical 主存储') : t('未命名存储'));
  const descriptionParts = [];
  if (profile.id === 'canonical' || profile.builtin) {
    descriptionParts.push(t('元数据与 zip 都由运行时主存储负责。'));
  } else if (profile.type === 's3') {
    descriptionParts.push(`${t('Bucket')}: ${firstNonEmpty(profile.s3_bucket, '-')}`);
    descriptionParts.push(`${t('Prefix')}: ${firstNonEmpty(profile.s3_prefix, '/')}`);
    if (firstNonEmpty(profile.base_url)) {
      descriptionParts.push(`${t('Base URL')}: ${profile.base_url}`);
    }
  } else if (profile.type === 'webdav') {
    descriptionParts.push(`${t('WebDAV Endpoint')}: ${firstNonEmpty(profile.webdav_endpoint, '-')}`);
    if (firstNonEmpty(profile.base_url)) {
      descriptionParts.push(`${t('Base URL')}: ${profile.base_url}`);
    }
  } else if (profile.type === 'ftp') {
    descriptionParts.push(`${t('FTP 地址')}: ${firstNonEmpty(profile.ftp_address, '-')}`);
    descriptionParts.push(`${t('Root Dir')}: ${firstNonEmpty(profile.ftp_root_dir, '/')}`);
    descriptionParts.push(`${t('FTP 安全模式')}: ${firstNonEmpty(profile.ftp_security, 'plain')}`);
    if (firstNonEmpty(profile.base_url)) {
      descriptionParts.push(`${t('Base URL')}: ${profile.base_url}`);
    }
  } else {
    descriptionParts.push(t('本地目录: {value}', { value: firstNonEmpty(profile.base_dir, '-') }));
    if (firstNonEmpty(profile.base_url)) {
      descriptionParts.push(`${t('Base URL')}: ${profile.base_url}`);
    }
  }
  if (profile.id === firstNonEmpty(defaultProfileID, 'canonical')) {
    descriptionParts.push(t('也是当前默认制品存储'));
  }

  return {
    type: 'info',
    message: t('当前落到 {value}', { value: label }),
    description: descriptionParts.join(' · '),
  };
}

export default function Publish() {
  const { t, locale } = useI18n();
  const [publishForm] = Form.useForm();
  const [syncForm] = Form.useForm();
  const selectedPublishStorageProfileID = Form.useWatch('artifact_storage_profile_id', publishForm);
  const selectedSyncStorageProfileID = Form.useWatch('sync_artifact_storage_profile_id', syncForm);
  const [file, setFile] = useState(null);
  const [fileList, setFileList] = useState([]);
  const [publishLoading, setPublishLoading] = useState(false);
  const [publishProgress, setPublishProgress] = useState(0);
  const [syncLoading, setSyncLoading] = useState(false);
  const [syncPreviewLoading, setSyncPreviewLoading] = useState(false);
  const [syncInspectLoading, setSyncInspectLoading] = useState(false);
  const [preview, setPreview] = useState(null);
  const [syncReleasePreview, setSyncReleasePreview] = useState(null);
  const [syncResult, setSyncResult] = useState(null);
  const [syncInspection, setSyncInspection] = useState(null);
  const [registryStatus, setRegistryStatus] = useState(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [artifactSettings, setArtifactSettings] = useState(null);
  const [reindexing, setReindexing] = useState(false);
  const [showSyncAdvanced, setShowSyncAdvanced] = useState(false);
  const uploadParseSeqRef = React.useRef(0);
  const lastResolvedSyncSourceUrlRef = React.useRef('');

  const resetPublishPreview = () => {
    setPreview(null);
    clearFieldErrors(publishForm, PUBLISH_FIELD_NAMES);
    publishForm.setFieldsValue({
      kind: undefined,
      name: undefined,
      version: undefined,
      title: undefined,
      summary: undefined,
      description: undefined,
    });
  };

  const clearPublishSelection = () => {
    setFile(null);
    setFileList([]);
    resetPublishPreview();
  };

  const buildSyncLookupPayload = (values) => ({
    owner: values.sync_owner,
    repo: values.sync_repo,
    tag: values.sync_tag,
    asset: values.sync_asset,
    api_base_url: values.sync_api_base_url,
    token: values.sync_token,
  });

  const artifactStorageProfiles = React.useMemo(() => {
    const profiles = artifactSettings?.artifact_storage?.profiles;
    return Array.isArray(profiles) ? profiles.filter((item) => item?.id) : [];
  }, [artifactSettings]);

  const defaultArtifactStorageProfileID = React.useMemo(
    () => firstNonEmpty(artifactSettings?.artifact_storage?.default_profile_id, 'canonical'),
    [artifactSettings]
  );

  const artifactStorageOptions = React.useMemo(() => {
    if (artifactStorageProfiles.length === 0) {
      return [{ value: 'canonical', label: t('Canonical 主存储') }];
    }
    return artifactStorageProfiles.map((profile) => ({
      value: profile.id,
      label: `${firstNonEmpty(profile.name, profile.id)} · ${String(profile.type || 'local').toUpperCase()}${profile.builtin ? ` · ${t('系统内置')}` : ''}`,
    }));
  }, [artifactStorageProfiles, t]);

  const artifactStorageProfileMap = React.useMemo(() => {
    const nextMap = {
      canonical: {
        id: 'canonical',
        name: t('Canonical 主存储'),
        type: 'canonical',
        builtin: true,
      },
    };
    artifactStorageProfiles.forEach((profile) => {
      if (profile?.id) {
        nextMap[profile.id] = profile;
      }
    });
    return nextMap;
  }, [artifactStorageProfiles, t]);

  const selectedPublishStorageProfile = React.useMemo(
    () => artifactStorageProfileMap[firstNonEmpty(selectedPublishStorageProfileID, defaultArtifactStorageProfileID, 'canonical')] || null,
    [artifactStorageProfileMap, defaultArtifactStorageProfileID, selectedPublishStorageProfileID]
  );

  const selectedSyncStorageProfile = React.useMemo(
    () => artifactStorageProfileMap[firstNonEmpty(selectedSyncStorageProfileID, defaultArtifactStorageProfileID, 'canonical')] || null,
    [artifactStorageProfileMap, defaultArtifactStorageProfileID, selectedSyncStorageProfileID]
  );

  const publishStorageAlert = React.useMemo(
    () => buildArtifactStorageAlert(selectedPublishStorageProfile, defaultArtifactStorageProfileID, t),
    [defaultArtifactStorageProfileID, selectedPublishStorageProfile, t]
  );

  const syncStorageAlert = React.useMemo(
    () => buildArtifactStorageAlert(selectedSyncStorageProfile, defaultArtifactStorageProfileID, t),
    [defaultArtifactStorageProfileID, selectedSyncStorageProfile, t]
  );

  React.useEffect(() => {
    let active = true;
    setStatusLoading(true);
    api.getRegistryStatus()
      .then((res) => {
        if (active) {
          setRegistryStatus(res.data.data || null);
        }
      })
      .catch((error) => {
        if (active) {
          message.error(getAPIErrorMessage(error, t('加载快照状态失败')));
        }
      })
      .finally(() => {
        if (active) {
          setStatusLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [t]);

  React.useEffect(() => {
    let active = true;
    setSettingsLoading(true);
    api.getSettings()
      .then((res) => {
        if (active) {
          setArtifactSettings(res?.data?.data || null);
        }
      })
      .catch((error) => {
        if (active) {
          message.error(getAPIErrorMessage(error, t('加载存储设置失败')));
        }
      })
      .finally(() => {
        if (active) {
          setSettingsLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [t]);

  React.useEffect(() => {
    const publishValue = firstNonEmpty(publishForm.getFieldValue('artifact_storage_profile_id'));
    const syncValue = firstNonEmpty(syncForm.getFieldValue('sync_artifact_storage_profile_id'));
    const nextDefault = firstNonEmpty(defaultArtifactStorageProfileID, 'canonical');
    if (!publishValue) {
      publishForm.setFieldsValue({ artifact_storage_profile_id: nextDefault });
    }
    if (!syncValue) {
      syncForm.setFieldsValue({ sync_artifact_storage_profile_id: nextDefault });
    }
  }, [defaultArtifactStorageProfileID, publishForm, syncForm]);

  const handleFileChange = async (nextFile, seq) => {
    setFile(nextFile);
    setFileList([nextFile]);
    resetPublishPreview();
    try {
      const zip = await JSZip.loadAsync(nextFile);
      const manifestFile = zip.file('manifest.json');
      if (!manifestFile) {
        if (uploadParseSeqRef.current === seq) {
          clearPublishSelection();
        }
        message.warning(t('zip 内缺少 manifest.json，无法用于发布'));
        return;
      }
      const content = await manifestFile.async('string');
      const manifest = JSON.parse(content);
      const coordinates = resolveManifestCoordinates(manifest);
      if (!coordinates.kind || !coordinates.name || !coordinates.version) {
        if (uploadParseSeqRef.current === seq) {
          clearPublishSelection();
        }
        message.warning(t('manifest.json 缺少可发布的 kind、name 或 version'));
        return;
      }
      if (uploadParseSeqRef.current !== seq) {
        return;
      }
      setPreview(manifest);
      publishForm.setFieldsValue({
        kind: coordinates.kind || undefined,
        name: coordinates.name,
        version: coordinates.version,
        title: firstNonEmpty(manifest.title, manifest.display_name),
        summary: firstNonEmpty(manifest.summary),
        description: firstNonEmpty(manifest.description, manifest.summary),
      });
    } catch (error) {
      if (uploadParseSeqRef.current === seq) {
        clearPublishSelection();
      }
      message.warning(getAPIErrorMessage(error, t('无法解析 manifest.json')));
    }
  };

  const handleBeforeUpload = (nextFile) => {
    uploadParseSeqRef.current += 1;
    const seq = uploadParseSeqRef.current;
    if (nextFile.size > MAX_ARTIFACT_FILE_SIZE) {
      clearPublishSelection();
      message.error(t('文件大小不能超过 100MB'));
      return Upload.LIST_IGNORE;
    }
    void handleFileChange(nextFile, seq);
    return false;
  };

  const onFinish = async (values) => {
    clearFieldErrors(publishForm, PUBLISH_FIELD_NAMES);
    if (!file) {
      message.error(t('请上传制品文件'));
      return;
    }

    if (file.size > MAX_ARTIFACT_FILE_SIZE) {
      message.error(t('文件大小不能超过 100MB'));
      return;
    }

    setPublishLoading(true);
    setPublishProgress(0);
    const formData = new FormData();
    formData.append('artifact', file);
    formData.append('kind', values.kind);
    formData.append('name', values.name);
    formData.append('version', values.version);
    formData.append('channel', values.channel);
    formData.append('artifact_storage_profile_id', firstNonEmpty(values.artifact_storage_profile_id, defaultArtifactStorageProfileID, 'canonical'));
    formData.append('metadata', JSON.stringify({
      title: values.title,
      summary: values.summary,
      description: values.description,
      release_notes: values.release_notes,
      publisher: { id: 'auralogic', name: 'AuraLogic' },
      labels: ['official'],
    }));

    try {
      const response = await api.publishRelease(formData, setPublishProgress);
      message.success(response?.data?.message || t('发布成功'));
      if (response?.data?.warning) {
        message.warning(response.data.warning);
      }
      setRegistryStatus(response?.data?.data?.status || registryStatus);
      publishForm.resetFields();
      publishForm.setFieldsValue({
        artifact_storage_profile_id: firstNonEmpty(values.artifact_storage_profile_id, defaultArtifactStorageProfileID, 'canonical'),
        channel: 'stable',
      });
      clearPublishSelection();
      setPublishProgress(0);
    } catch (error) {
      const apiMessage = getAPIErrorMessage(error, t('发布失败'));
      applyFieldErrors(publishForm, buildPublishFieldErrors(apiMessage, t));
      message.error(apiMessage);
    } finally {
      setPublishLoading(false);
    }
  };

  const handleSyncSubmit = async (values) => {
    clearFieldErrors(syncForm, SYNC_FIELD_NAMES);
    setSyncLoading(true);
    setSyncResult(null);

    try {
      const response = await api.syncGitHubRelease({
        kind: values.sync_kind,
        name: values.sync_name,
        version: values.sync_version,
        channel: values.sync_channel,
        artifact_storage_profile_id: firstNonEmpty(values.sync_artifact_storage_profile_id, defaultArtifactStorageProfileID, 'canonical'),
        owner: values.sync_owner,
        repo: values.sync_repo,
        tag: values.sync_tag,
        asset: values.sync_asset,
        api_base_url: values.sync_api_base_url,
        token: values.sync_token,
        metadata: {
          title: values.sync_title,
          summary: values.sync_summary,
          description: values.sync_description,
          release_notes: values.sync_release_notes,
        },
      });

      message.success(response?.data?.message || t('同步成功'));
      if (response?.data?.warning) {
        message.warning(response.data.warning);
      }

      setSyncResult(response?.data?.data?.result || null);
      setRegistryStatus(response?.data?.data?.status || registryStatus);
      syncForm.setFieldsValue({
        sync_kind: response?.data?.data?.result?.kind || values.sync_kind,
        sync_name: response?.data?.data?.result?.name || values.sync_name,
        sync_version: response?.data?.data?.result?.version || values.sync_version,
        sync_artifact_storage_profile_id: response?.data?.data?.result?.artifact_storage_profile_id || values.sync_artifact_storage_profile_id,
        sync_token: '',
      });
    } catch (error) {
      const apiMessage = getAPIErrorMessage(error, t('同步失败'));
      const fieldErrors = buildSyncFieldErrors(apiMessage, t);
      if (shouldRevealSyncAdvanced(fieldErrors)) {
        setShowSyncAdvanced(true);
      }
      applyFieldErrors(syncForm, fieldErrors);
      message.error(apiMessage);
    } finally {
      setSyncLoading(false);
    }
  };

  const handlePreviewSyncRelease = async (overrides = {}, options = {}) => {
    clearFieldErrors(syncForm, SYNC_FIELD_NAMES);
    setSyncPreviewLoading(true);

    const values = { ...syncForm.getFieldsValue(), ...overrides };
    try {
      const response = await api.previewGitHubRelease(buildSyncLookupPayload(values));
      const result = response?.data?.data?.result || null;
      setSyncReleasePreview(result);
      const nextFieldValues = {};
      if (result?.selected_asset) {
        nextFieldValues.sync_asset = result.selected_asset;
      }
      if (!firstNonEmpty(values.sync_title) && firstNonEmpty(result?.release_name)) {
        nextFieldValues.sync_title = result.release_name;
      }
      if (!firstNonEmpty(values.sync_release_notes) && firstNonEmpty(result?.release_body)) {
        nextFieldValues.sync_release_notes = result.release_body;
      }
      if (Object.keys(nextFieldValues).length > 0) {
        syncForm.setFieldsValue(nextFieldValues);
      }
      if (!options.silentSuccess) {
        message.success(response?.data?.message || t('远端 Release 已加载'));
      }
      return true;
    } catch (error) {
      setSyncReleasePreview(null);
      const apiMessage = getAPIErrorMessage(error, t('加载远端 Release 失败'));
      const fieldErrors = buildSyncFieldErrors(apiMessage, t);
      if (shouldRevealSyncAdvanced(fieldErrors)) {
        setShowSyncAdvanced(true);
      }
      applyFieldErrors(syncForm, fieldErrors);
      if (!options.silentError) {
        message.error(apiMessage);
      }
      return false;
    } finally {
      setSyncPreviewLoading(false);
    }
  };

  const applySyncSourceURL = async (rawValue, options = {}) => {
    const trimmed = firstNonEmpty(rawValue);
    if (!trimmed) {
      lastResolvedSyncSourceUrlRef.current = '';
      clearFieldErrors(syncForm, ['sync_source_url']);
      return false;
    }
    if (!options.force && trimmed === lastResolvedSyncSourceUrlRef.current) {
      return true;
    }

    const parsed = parseGitHubReleaseURL(trimmed);
    if (!parsed) {
      syncForm.setFields([{ name: 'sync_source_url', errors: [t('请输入 GitHub Release 页面或资产下载链接')] }]);
      if (options.notifyInvalid) {
        message.warning(t('当前链接不是受支持的 GitHub Release 页面或资产下载链接'));
      }
      return false;
    }

    lastResolvedSyncSourceUrlRef.current = trimmed;
    const nextValues = {
      sync_source_url: trimmed,
      sync_owner: parsed.owner,
      sync_repo: parsed.repo,
      sync_tag: parsed.tag,
      sync_asset: parsed.asset || undefined,
      sync_api_base_url: parsed.apiBaseURL || undefined,
    };
    syncForm.setFieldsValue(nextValues);
    setSyncReleasePreview(null);
    setSyncInspection(null);
    clearFieldErrors(syncForm, [...SYNC_SOURCE_FIELD_NAMES, 'sync_kind', 'sync_name', 'sync_version']);
    const loaded = await handlePreviewSyncRelease(nextValues, {
      silentSuccess: options.silentSuccess,
      silentError: options.silentError,
    });
    if (!loaded) {
      lastResolvedSyncSourceUrlRef.current = '';
    }
    return true;
  };

  const handleInspectSyncTarget = async () => {
    clearFieldErrors(syncForm, SYNC_FIELD_NAMES);
    setSyncInspectLoading(true);

    const values = syncForm.getFieldsValue();
    try {
      const response = await api.inspectGitHubRelease({
        kind: values.sync_kind,
        name: values.sync_name,
        version: values.sync_version,
        owner: values.sync_owner,
        repo: values.sync_repo,
        tag: values.sync_tag,
        asset: values.sync_asset,
        api_base_url: values.sync_api_base_url,
        token: values.sync_token,
      });
      const result = response?.data?.data?.result || null;
      setSyncInspection(result);
      const mismatchFieldErrors = buildSyncInspectionMismatchFieldErrors(result, t);
      if (mismatchFieldErrors.length > 0) {
        setShowSyncAdvanced(true);
        syncForm.setFields(mismatchFieldErrors);
        message.warning(t('远端包已解析，但你手工填写的坐标与远端 manifest 不一致'));
      } else {
        clearFieldErrors(syncForm, ['sync_kind', 'sync_name', 'sync_version']);
        message.success(response?.data?.message || t('远端检查成功'));
      }
    } catch (error) {
      setSyncInspection(null);
      const apiMessage = getAPIErrorMessage(error, t('远端检查失败'));
      const fieldErrors = buildSyncFieldErrors(apiMessage, t);
      if (shouldRevealSyncAdvanced(fieldErrors)) {
        setShowSyncAdvanced(true);
      }
      applyFieldErrors(syncForm, fieldErrors);
      message.error(apiMessage);
    } finally {
      setSyncInspectLoading(false);
    }
  };

  const handleReindex = async () => {
    setReindexing(true);
    try {
      const response = await api.rebuildRegistry();
      message.success(response?.data?.message || t('重建成功'));
      setRegistryStatus(response?.data?.data?.status || null);
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('重建快照失败')));
    } finally {
      setReindexing(false);
    }
  };

  const summaryTiles = [
    {
      key: 'package',
      label: t('本地包发布'),
      value: preview ? `${preview.name || '-'}@${preview.version || '-'}` : t('待选择'),
      note: preview ? t('manifest 已解析，发布坐标已锁定') : t('上传 zip 后自动读取 manifest.json'),
    },
    {
      key: 'sync',
      label: t('远端 Release 预览'),
      value: syncReleasePreview?.selected_asset || t('未加载'),
      note: syncReleasePreview ? t('已发现 {count} 个资产', { count: syncReleasePreview.asset_count ?? 0 }) : t('先加载 owner / repo / tag 对应的 Release'),
    },
    {
      key: 'inspect',
      label: t('远端包检查'),
      value: syncInspection ? (syncInspection.changed ? t('存在差异') : t('已通过')) : t('未检查'),
      note: syncInspection ? `${syncInspection.kind || '-'}:${syncInspection.name || '-'}:${syncInspection.version || '-'}` : t('同步前可先做坐标与包内容校验'),
    },
    {
      key: 'registry',
      label: t('快照状态'),
      value: formatRegistryStatus(registryStatus?.status || 'idle', t),
      note: registryStatus?.message || t('发布前建议先确认快照健康度'),
    },
  ];

  return (
    <div className="market-page">
      <PageHeader
        eyebrow={t('发布工作流')}
        title={t('发布与同步')}
        description={t('先确认快照状态，再选择本地发布或远端同步。')}
        meta={[
          { label: t('最大包体'), value: '100MB' },
          { label: t('支持来源'), value: t('本地 ZIP / GitHub Release') },
          { label: t('快照状态'), value: formatRegistryStatus(registryStatus?.status || 'idle', t) },
        ]}
      />

      <div className="market-summary-grid">
        {summaryTiles.map((item) => (
          <div key={item.key} className="market-summary-tile">
            <div className="market-summary-label">{item.label}</div>
            <div className="market-summary-value" style={{ fontSize: item.value.length > 14 ? 18 : 28 }}>
              {item.value}
            </div>
            <div className="market-summary-note">{item.note}</div>
          </div>
        ))}
      </div>

      <RegistryStatusCard
        title={t('发布前快照检查')}
        status={registryStatus}
        loading={statusLoading}
        reindexing={reindexing}
        onReindex={handleReindex}
      />

      <Card className="market-panel" title={t('发布新版本')} extra={<span className="market-inline-note">{t('本地 zip 会以包内 manifest 坐标为准')}</span>}>
        <Form form={publishForm} layout="vertical" onFinish={onFinish}>
          <div className="market-section-header">
            <div>
              <h3 className="market-section-title">{t('包坐标与预览')}</h3>
              <p className="market-section-description">{t('先解析 manifest，再确认是否发布。')}</p>
            </div>
          </div>
          <Form.Item
            label={t('上传文件')}
            required
            extra={t('系统会读取 zip 内 manifest.json，并以其中的 kind / name / version 为准。解析成功后这三个字段会锁定。')}
          >
            <Upload.Dragger
              accept=".zip"
              maxCount={1}
              fileList={fileList}
              beforeUpload={handleBeforeUpload}
              onRemove={() => {
                uploadParseSeqRef.current += 1;
                clearPublishSelection();
              }}
            >
              <p className="ant-upload-drag-icon"><InboxOutlined /></p>
              <p>{t('拖拽 zip 文件到这里或点击上传')}</p>
              <p className="market-upload-hint">{t('最大 100MB，仅支持标准 zip 制品包')}</p>
            </Upload.Dragger>
          </Form.Item>
        {preview && (
          <Card className="market-panel market-modal-summary" size="small" title={t('Manifest 预览')} style={{ marginBottom: 16 }}>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message={t('当前发布坐标已从包内 manifest.json 读取并锁定，若需修改，请重新打包后再上传。')}
            />
            <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
              <Descriptions.Item label={t('名称')}>{preview.name}</Descriptions.Item>
              <Descriptions.Item label={t('版本')}>{preview.version}</Descriptions.Item>
              <Descriptions.Item label={t('类型')}>{resolveManifestCoordinates(preview).kind || publishForm.getFieldValue('kind') || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('协议版本')}>{preview.protocol_version}</Descriptions.Item>
              <Descriptions.Item label={t('入口')}>{preview.entry || preview.address || '-'}</Descriptions.Item>
              </Descriptions>
          </Card>
        )}
          <Row gutter={16}>
            <Col xs={24} md={12} xl={8}>
              <Form.Item name="kind" label={t('制品类型')} rules={[{ required: true }]}>
                <Select disabled={Boolean(preview)}>
                  <Select.Option value="plugin_package">{t('插件包')}</Select.Option>
                  <Select.Option value="payment_package">{t('支付包')}</Select.Option>
                  <Select.Option value="email_template">{t('邮件模板')}</Select.Option>
                  <Select.Option value="landing_page_template">{t('落地页模板')}</Select.Option>
                  <Select.Option value="invoice_template">{t('账单模板')}</Select.Option>
                  <Select.Option value="auth_branding_template">{t('认证页品牌模板')}</Select.Option>
                  <Select.Option value="page_rule_pack">{t('页面规则包')}</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col xs={24} md={12} xl={8}>
              <Form.Item name="name" label={t('制品名称')} rules={[{ required: true }]}>
                <Input placeholder={t('例如: hello-market')} disabled={Boolean(preview)} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} xl={4}>
              <Form.Item name="version" label={t('版本号')} rules={[{ required: true }]}>
                <Input placeholder={t('例如: 1.0.0')} disabled={Boolean(preview)} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} xl={4}>
              <Form.Item name="channel" label={t('发布渠道')} initialValue="stable" rules={[{ required: true }]}>
                <Select>
                  <Select.Option value="stable">Stable</Select.Option>
                  <Select.Option value="beta">Beta</Select.Option>
                  <Select.Option value="alpha">Alpha</Select.Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} lg={12}>
              <Form.Item
                name="artifact_storage_profile_id"
                label={t('制品存储')}
                extra={t('选择 zip 包的存储位置；manifest、origin 和索引仍写入 canonical 主存储。')}
              >
                <Select
                  options={artifactStorageOptions}
                  loading={settingsLoading}
                  showSearch
                  optionFilterProp="label"
                  placeholder={t('选择制品存储')}
                />
              </Form.Item>
              <Alert
                type={publishStorageAlert.type}
                showIcon
                style={{ marginBottom: 16 }}
                message={publishStorageAlert.message}
                description={publishStorageAlert.description}
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} lg={12}>
              <Form.Item name="title" label={t('标题')} rules={[{ required: true }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col xs={24} lg={12}>
              <Form.Item name="summary" label={t('摘要')}>
                <Input.TextArea rows={2} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="description" label={t('描述')}>
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item name="release_notes" label={t('发布说明')}>
            <Input.TextArea rows={3} />
          </Form.Item>
        {publishLoading && publishProgress > 0 && (
          <Progress percent={publishProgress} status="active" style={{ marginBottom: 16 }} />
        )}
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={publishLoading} size="large">
              {t('发布')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
      <Card className="market-panel" title={t('同步 GitHub Release')} extra={<span className="market-inline-note">{t('先预览，再检查，最后同步')}</span>}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('从 GitHub Release 拉取 zip 资产并同步到所选制品存储。kind、name、version 可留空；如果手动填写，则必须与远端包内 manifest 保持一致。')}
        />
        <Form
          form={syncForm}
          layout="vertical"
          onFinish={handleSyncSubmit}
          onValuesChange={(changedValues) => {
            const changedKeys = Object.keys(changedValues);
            if (changedKeys.includes('sync_source_url') && !firstNonEmpty(changedValues.sync_source_url)) {
              lastResolvedSyncSourceUrlRef.current = '';
              clearFieldErrors(syncForm, ['sync_source_url']);
            }
            if (changedKeys.some((key) => SYNC_SOURCE_FIELD_NAMES.includes(key)) && !changedKeys.includes('sync_source_url')) {
              lastResolvedSyncSourceUrlRef.current = '';
            }
            if (changedKeys.includes('sync_token')) {
              lastResolvedSyncSourceUrlRef.current = '';
            }
            if (changedKeys.some((key) => SYNC_PREVIEW_DEPENDENCY_FIELDS.has(key))) {
              setSyncReleasePreview(null);
            }
            if (changedKeys.some((key) => SYNC_INSPECTION_DEPENDENCY_FIELDS.has(key))) {
              setSyncInspection(null);
              clearFieldErrors(syncForm, ['sync_kind', 'sync_name', 'sync_version']);
            }
          }}
          initialValues={{ sync_channel: 'stable' }}
        >
          <Descriptions size="small" column={DESCRIPTION_COLUMNS} style={{ marginBottom: 16 }}>
            <Descriptions.Item label={t('最少必填')}>{t('GitHub 链接 或 owner / repo / tag')}</Descriptions.Item>
            <Descriptions.Item label={t('支持场景')}>{t('GitHub.com 与 GitHub Enterprise API')}</Descriptions.Item>
          </Descriptions>
          <div className="market-section-header">
            <div>
              <h3 className="market-section-title">{t('URL 一键解析优先')}</h3>
              <p className="market-section-description">{t('大多数场景下，只需要粘贴链接、确认资产，必要时填写 Token。')}</p>
            </div>
          </div>
          <Form.Item
            name="sync_source_url"
            label={t('GitHub Release 地址')}
            extra={t('支持 Release 页面链接和 releases/download 资产链接。粘贴后失焦会自动解析，也可回车立即加载。')}
          >
            <Input.Search
              allowClear
              enterButton={t('解析并加载')}
              placeholder={t('例如: https://github.com/owner/repo/releases/tag/v1.0.0')}
              onSearch={(value) => {
                void applySyncSourceURL(value, { notifyInvalid: true });
              }}
              onBlur={(event) => {
                void applySyncSourceURL(event.target.value, { silentSuccess: true, silentError: true });
              }}
            />
          </Form.Item>
          {syncReleasePreview && (
            <Alert
              type="success"
              showIcon
              style={{ marginBottom: 16 }}
              message={t('已自动回填 owner / repo / tag，可直接继续。')}
              description={t('只有在需要修正仓库坐标、启用 GitHub Enterprise 或覆盖展示文案时，才需要展开高级设置。')}
            />
          )}
          <Row gutter={16}>
            <Col xs={24} xl={14}>
              <Form.Item name="sync_asset" label={t('Asset 文件名')} rules={[{ required: true, message: t('请输入资产文件名') }]}>
                {Array.isArray(syncReleasePreview?.assets) && syncReleasePreview.assets.length > 0 ? (
                  <Select
                    showSearch
                    optionFilterProp="label"
                    placeholder={t('先加载 Release 资产或手动输入')}
                    options={syncReleasePreview.assets.map((asset) => ({
                      value: asset.name,
                      label: `${asset.name} (${formatBytes(asset.size)} / ${asset.content_type || '-'})`,
                    }))}
                  />
                ) : (
                  <Input placeholder={t('例如: hello-market-1.0.0.zip')} />
                )}
              </Form.Item>
            </Col>
            <Col xs={24} xl={10}>
              <Form.Item name="sync_token" label={t('访问令牌')}>
                <Input.Password placeholder={t('私有仓库可选；仅本次请求使用')} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} xl={14}>
              <Form.Item
                name="sync_artifact_storage_profile_id"
                label={t('制品存储')}
                extra={t('同步得到的 zip 包会写入这里；元数据仍写入 canonical 主存储。')}
              >
                <Select
                  options={artifactStorageOptions}
                  loading={settingsLoading}
                  showSearch
                  optionFilterProp="label"
                  placeholder={t('选择制品存储')}
                />
              </Form.Item>
              <Alert
                type={syncStorageAlert.type}
                showIcon
                style={{ marginBottom: 16 }}
                message={syncStorageAlert.message}
                description={syncStorageAlert.description}
              />
            </Col>
          </Row>
          <div className="market-advanced-toggle">
            <div>
              <div className="market-advanced-title">{t('高级设置')}</div>
              <div className="market-advanced-description">{t('手动坐标、校验字段和元数据覆盖')}</div>
            </div>
            <Button type="link" onClick={() => setShowSyncAdvanced((current) => !current)}>
              {showSyncAdvanced ? t('收起高级设置') : t('展开高级设置')}
            </Button>
          </div>
          {showSyncAdvanced && (
            <div className="market-advanced-panel">
              <div className="market-section-header">
                <div>
                  <h3 className="market-section-title">{t('手动源坐标')}</h3>
                  <p className="market-section-description">{t('用于 GitHub Enterprise、手动坐标输入，或不使用链接解析时直接指定仓库。')}</p>
                </div>
              </div>
              <Row gutter={16}>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_owner" label={t('GitHub 所有者')} rules={[{ required: true, message: t('请输入 owner') }]}>
                    <Input placeholder={t('例如: auralogic')} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_repo" label={t('GitHub 仓库')} rules={[{ required: true, message: t('请输入 repo') }]}>
                    <Input placeholder={t('例如: market-packages')} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_tag" label={t('Release 标签')} rules={[{ required: true, message: t('请输入 tag') }]}>
                    <Input placeholder={t('例如: 1.0.0')} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_api_base_url" label={t('GitHub API 地址')}>
                    <Input placeholder={t('默认 api.github.com')} />
                  </Form.Item>
                </Col>
              </Row>
              <div className="market-section-header">
                <div>
                  <h3 className="market-section-title">{t('可选校验')}</h3>
                  <p className="market-section-description">{t('kind / name / version 留空时会自动识别；手动填写后会与远端 manifest 做严格比对。')}</p>
                </div>
              </div>
              <Row gutter={16}>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_kind" label={t('制品类型')}>
                    <Select allowClear placeholder={t('留空则自动识别')}>
                      <Select.Option value="plugin_package">{t('插件包')}</Select.Option>
                      <Select.Option value="payment_package">{t('支付包')}</Select.Option>
                      <Select.Option value="email_template">{t('邮件模板')}</Select.Option>
                      <Select.Option value="landing_page_template">{t('落地页模板')}</Select.Option>
                      <Select.Option value="invoice_template">{t('账单模板')}</Select.Option>
                      <Select.Option value="auth_branding_template">{t('认证页品牌模板')}</Select.Option>
                      <Select.Option value="page_rule_pack">{t('页面规则包')}</Select.Option>
                    </Select>
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_name" label={t('制品名称')}>
                    <Input placeholder={t('留空则从远端 manifest 读取')} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_version" label={t('版本号')}>
                    <Input placeholder={t('留空则从远端 manifest 读取')} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Form.Item name="sync_channel" label={t('发布渠道')} rules={[{ required: true }]}>
                    <Select>
                      <Select.Option value="stable">Stable</Select.Option>
                      <Select.Option value="beta">Beta</Select.Option>
                      <Select.Option value="alpha">Alpha</Select.Option>
                    </Select>
                  </Form.Item>
                </Col>
              </Row>
              <div className="market-section-header">
                <div>
                  <h3 className="market-section-title">{t('同步元数据覆盖')}</h3>
                  <p className="market-section-description">{t('导入到市场源后展示的标题、摘要和发布说明，可按需覆盖。')}</p>
                </div>
              </div>
              <Row gutter={16}>
                <Col xs={24} lg={12}>
                  <Form.Item name="sync_title" label={t('标题')}>
                    <Input placeholder={t('留空则回退到 Release 标题或远端 manifest')} />
                  </Form.Item>
                </Col>
                <Col xs={24} lg={12}>
                  <Form.Item name="sync_summary" label={t('摘要')}>
                    <Input.TextArea rows={2} placeholder={t('可选')} />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item name="sync_description" label={t('描述')}>
                <Input.TextArea rows={4} placeholder={t('可选')} />
              </Form.Item>
              <Form.Item name="sync_release_notes" label={t('发布说明')} style={{ marginBottom: 0 }}>
                <Input.TextArea rows={3} placeholder={t('留空则回退到 GitHub Release 正文')} />
              </Form.Item>
            </div>
          )}
          {syncReleasePreview && (
            <Card className="market-panel market-modal-summary" size="small" title={t('远端 Release 预览')} style={{ marginBottom: 16 }}>
              <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
                <Descriptions.Item label={t('标题')}>{syncReleasePreview.release_name || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('资产数')}>{syncReleasePreview.asset_count ?? 0}</Descriptions.Item>
                <Descriptions.Item label={t('发布时间')}>{formatDateTime(syncReleasePreview.published_at, locale)}</Descriptions.Item>
                <Descriptions.Item label={t('默认资产')}>{syncReleasePreview.selected_asset || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('浏览地址')} span={2}>
                  {syncReleasePreview.browser_url ? (
                    <Typography.Link href={syncReleasePreview.browser_url} target="_blank" rel="noreferrer">
                      {syncReleasePreview.browser_url}
                    </Typography.Link>
                  ) : '-'}
                </Descriptions.Item>
                <Descriptions.Item label={t('发布说明')} span={2}>
                  <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }} ellipsis={{ rows: 3, expandable: true, symbol: t('展开') }}>
                    {syncReleasePreview.release_body || '-'}
                  </Typography.Paragraph>
                </Descriptions.Item>
              </Descriptions>
            </Card>
          )}
          {syncInspection && (
            <Card className="market-panel market-modal-summary" size="small" title={t('远端包检查结果')} style={{ marginBottom: 16 }}>
              <Alert
                type={syncInspection.changed ? 'warning' : 'success'}
                showIcon
                style={{ marginBottom: 16 }}
                message={syncInspection.changed ? t('远端包可读取，但手工填写的坐标与远端 manifest 不一致') : t('远端包检查通过，可直接同步')}
                description={syncInspection.changed
                  ? t('不一致字段: {fields}', { fields: (syncInspection.changed_fields || []).join(', ') || 'unknown' })
                  : t('已解析为 {value}', { value: `${syncInspection.kind}:${syncInspection.name}:${syncInspection.version}` })}
              />
              <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
                <Descriptions.Item label={t('解析类型')}>{syncInspection.kind || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('解析名称')}>{syncInspection.name || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('解析版本')}>{syncInspection.version || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('资产大小')}>{formatBytes(syncInspection.asset_size)}</Descriptions.Item>
                <Descriptions.Item label={t('发布时间')}>{formatDateTime(syncInspection.published_at, locale)}</Descriptions.Item>
                <Descriptions.Item label={t('远端更新时间')}>{formatDateTime(syncInspection.asset_updated_at, locale)}</Descriptions.Item>
                <Descriptions.Item label={t('SHA256')} span={2}>
                  <Typography.Text copyable ellipsis={{ tooltip: syncInspection.sha256 }} style={{ maxWidth: 420 }}>
                    {syncInspection.sha256 || '-'}
                  </Typography.Text>
                </Descriptions.Item>
                <Descriptions.Item label={t('浏览地址')} span={2}>
                  {syncInspection.browser_url ? (
                    <Typography.Link href={syncInspection.browser_url} target="_blank" rel="noreferrer">
                      {syncInspection.browser_url}
                    </Typography.Link>
                  ) : '-'}
                </Descriptions.Item>
              </Descriptions>
            </Card>
          )}
          <Form.Item>
            <Space size={12} wrap>
              <Button onClick={handlePreviewSyncRelease} loading={syncPreviewLoading} size="large">
                {t('加载 Release 资产')}
              </Button>
              <Button onClick={handleInspectSyncTarget} loading={syncInspectLoading} size="large">
                {t('检查远端包')}
              </Button>
              <Button type="primary" htmlType="submit" loading={syncLoading} size="large">
                {t('同步 Release')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
        {syncResult && (
          <Card className="market-panel market-modal-summary" size="small" title={t('最近一次同步结果')}>
            <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
              <Descriptions.Item label={t('坐标')}>
                {`${syncResult.kind}:${syncResult.name}:${syncResult.version}`}
              </Descriptions.Item>
              <Descriptions.Item label={t('渠道')}>{syncResult.channel || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('仓库')}>{`${syncResult.owner}/${syncResult.repo}`}</Descriptions.Item>
              <Descriptions.Item label={t('标签')}>{syncResult.tag}</Descriptions.Item>
              <Descriptions.Item label={t('资产')}>{syncResult.asset_name}</Descriptions.Item>
              <Descriptions.Item label={t('大小')}>{formatBytes(syncResult.asset_size)}</Descriptions.Item>
              <Descriptions.Item label={t('SHA256')}>
                <Typography.Text copyable ellipsis={{ tooltip: syncResult.sha256 }} style={{ maxWidth: 420 }}>
                  {syncResult.sha256 || '-'}
                </Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('浏览地址')}>
                {syncResult.browser_url ? (
                  <Typography.Link href={syncResult.browser_url} target="_blank" rel="noreferrer">
                    {syncResult.browser_url}
                  </Typography.Link>
                ) : '-'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        )}
      </Card>
    </div>
  );
}
