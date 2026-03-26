import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { api, getAPIErrorMessage } from '../api/client';
import PageHeader from '../components/PageHeader';
import { useI18n } from '../i18n';

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

function normalizeSearchTokens(value) {
  return String(value || '')
    .toLowerCase()
    .split(/[^a-z0-9]+/i)
    .map((item) => item.trim())
    .filter(Boolean);
}

function matchesArtifactSearch(record, queryValue) {
  const queryTokens = normalizeSearchTokens(queryValue);
  if (!queryTokens.length) {
    return true;
  }

  const searchTokens = [
    record?.name,
    record?.title,
    record?.summary,
    record?.kind,
    getTransportProvider(record) || 'local',
    ...(Array.isArray(record?.channels) ? record.channels : []),
  ].flatMap((value) => normalizeSearchTokens(value));

  if (!searchTokens.length) {
    return false;
  }

  return queryTokens.every((token) => searchTokens.includes(token));
}

function getTransportProvider(record) {
  return record?.latest_transport?.provider || record?.transport?.provider || record?.latest_origin?.provider || record?.origin?.provider || '';
}

function getTransportMode(record) {
  return record?.latest_transport?.mode || record?.transport?.mode || record?.latest_origin?.mode || record?.origin?.mode || '';
}

function getSyncTimestamp(record) {
  return record?.latest_sync?.last_synced_at || record?.sync?.last_synced_at || record?.latest_origin?.sync?.last_synced_at || record?.origin?.sync?.last_synced_at || '';
}

function renderTransportTag(provider, t) {
  const normalized = String(provider || '').trim();
  if (!normalized) {
    return <Tag>{t('本地')}</Tag>;
  }
  if (normalized === 'github_release') {
    return <Tag color="geekblue">GitHub Release</Tag>;
  }
  if (normalized === 'local') {
    return <Tag color="green">{t('本地')}</Tag>;
  }
  return <Tag>{normalized}</Tag>;
}

function renderModeTag(mode, t) {
  const normalized = String(mode || '').trim();
  if (!normalized) {
    return '-';
  }
  if (normalized === 'mirror') {
    return <Tag color="success">{t('镜像')}</Tag>;
  }
  if (normalized === 'proxy') {
    return <Tag color="processing">{t('代理')}</Tag>;
  }
  if (normalized === 'redirect') {
    return <Tag color="warning">{t('重定向')}</Tag>;
  }
  return <Tag>{normalized}</Tag>;
}

function renderChannelTags(record) {
  const channels = Array.isArray(record?.channels) && record.channels.length
    ? record.channels
    : (record?.channel ? [record.channel] : []);
  if (!channels.length) {
    return '-';
  }
  return (
    <Space size={4} wrap>
      {channels.map((channel) => (
        <Tag color="blue" key={channel}>{channel}</Tag>
      ))}
    </Space>
  );
}

function renderCopyableText(value, fallback = '-') {
  if (!value) {
    return fallback;
  }
  return (
    <Typography.Text copyable ellipsis={{ tooltip: value }} style={{ maxWidth: 280 }}>
      {value}
    </Typography.Text>
  );
}

function inspectionAlertProps(summary, t) {
  if (!summary) {
    return null;
  }
  if ((summary.failed_versions || 0) > 0) {
    return {
      type: 'warning',
      message: t('检查完成，但有 {count} 个版本检查失败', { count: summary.failed_versions }),
      description: t('已检查 {checked} 个 GitHub 版本，发现 {changed} 个版本与远端不一致。', {
        checked: summary.checked_versions || 0,
        changed: summary.changed_versions || 0,
      }),
    };
  }
  if ((summary.changed_versions || 0) > 0) {
    return {
      type: 'warning',
      message: t('发现 {count} 个版本与远端不一致', { count: summary.changed_versions }),
      description: t('已检查 {count} 个 GitHub 版本。', { count: summary.checked_versions || 0 }),
    };
  }
  return {
    type: 'success',
    message: t('所有已检查 GitHub 版本都与当前缓存一致'),
    description: t('已检查 {count} 个 GitHub 版本。', { count: summary.checked_versions || 0 }),
  };
}

const DESCRIPTION_COLUMNS = { xs: 1, md: 2 };

export default function Artifacts() {
  const { t, locale } = useI18n();
  const [artifacts, setArtifacts] = useState([]);
  const [loading, setLoading] = useState(false);
  const [selectedArtifact, setSelectedArtifact] = useState(null);
  const [versions, setVersions] = useState([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [query, setQuery] = useState('');
  const [kindFilter, setKindFilter] = useState('all');
  const [providerFilter, setProviderFilter] = useState('all');
  const [remoteToken, setRemoteToken] = useState('');
  const [checkingArtifact, setCheckingArtifact] = useState(false);
  const [artifactInspectionSummary, setArtifactInspectionSummary] = useState(null);
  const [inspectionByVersion, setInspectionByVersion] = useState({});
  const [checkingVersion, setCheckingVersion] = useState('');
  const [resyncingVersion, setResyncingVersion] = useState('');
  const [deletingArtifactKey, setDeletingArtifactKey] = useState('');
  const [deletingVersionKey, setDeletingVersionKey] = useState('');
  const [expandedRowKeys, setExpandedRowKeys] = useState([]);
  const [versionDetails, setVersionDetails] = useState({});
  const [versionDetailLoading, setVersionDetailLoading] = useState({});

  const loadArtifacts = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.listArtifacts();
      const catalog = res.data.data;
      const list = [];
      Object.keys(catalog).forEach((kind) => {
        if (catalog[kind]) {
          Object.keys(catalog[kind]).forEach((name) => {
            list.push({ kind, name, ...catalog[kind][name] });
          });
        }
      });
      setArtifacts(list);
    } catch (error) {
      setArtifacts([]);
      message.error(getAPIErrorMessage(error, t('加载制品列表失败')));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadArtifacts();
  }, [loadArtifacts]);

  const loadArtifactVersions = async (kind, name) => {
    const res = await api.getArtifactVersions(kind, name);
    return res.data.data;
  };

  const loadArtifactRelease = async (kind, name, version) => {
    const res = await api.getArtifactRelease(kind, name, version);
    return res.data.data;
  };

  const resetInspectionState = () => {
    setArtifactInspectionSummary(null);
    setInspectionByVersion({});
  };

  const resetVersionDetailState = () => {
    setExpandedRowKeys([]);
    setVersionDetails({});
    setVersionDetailLoading({});
  };

  const resetModalState = () => {
    setModalVisible(false);
    setSelectedArtifact(null);
    setVersions([]);
    setVersionsLoading(false);
    setRemoteToken('');
    resetInspectionState();
    resetVersionDetailState();
  };

  const viewVersions = async (record) => {
    setModalVisible(true);
    setSelectedArtifact({ ...record });
    setVersions([]);
    setVersionsLoading(true);
    setRemoteToken('');
    resetInspectionState();
    resetVersionDetailState();

    try {
      const artifact = await loadArtifactVersions(record.kind, record.name);
      setSelectedArtifact({ kind: record.kind, name: record.name, ...artifact });
      setVersions(artifact.releases || []);
    } catch (error) {
      setVersions([]);
      message.error(getAPIErrorMessage(error, t('加载版本历史失败')));
    } finally {
      setVersionsLoading(false);
    }
  };

  const ensureVersionDetail = async (record) => {
    if (!selectedArtifact || !record?.version) {
      return;
    }
    if (versionDetails[record.version] || versionDetailLoading[record.version]) {
      return;
    }

    setVersionDetailLoading((current) => ({
      ...current,
      [record.version]: true,
    }));
    try {
      const detail = await loadArtifactRelease(selectedArtifact.kind, selectedArtifact.name, record.version);
      setVersionDetails((current) => ({
        ...current,
        [record.version]: detail,
      }));
    } catch (error) {
      setExpandedRowKeys((current) => current.filter((item) => item !== record.version));
      message.error(getAPIErrorMessage(error, t('加载版本 {version} 详情失败', { version: record.version })));
    } finally {
      setVersionDetailLoading((current) => {
        const next = { ...current };
        delete next[record.version];
        return next;
      });
    }
  };

  const handleVersionExpand = (expanded, record) => {
    if (!record?.version) {
      return;
    }
    if (expanded) {
      setExpandedRowKeys((current) => (current.includes(record.version) ? current : [...current, record.version]));
      void ensureVersionDetail(record);
      return;
    }
    setExpandedRowKeys((current) => current.filter((item) => item !== record.version));
  };

  const handleCheckArtifactOrigins = async () => {
    if (!selectedArtifact) {
      return;
    }
    setCheckingArtifact(true);
    try {
      const response = await api.checkArtifactOrigins(selectedArtifact.kind, selectedArtifact.name, {
        token: remoteToken,
      });
      const payload = response?.data?.data || {};
      const nextInspectionByVersion = {};
      for (const item of payload.items || []) {
        if (item?.ok && item?.version && item?.result) {
          nextInspectionByVersion[item.version] = item.result;
        }
      }
      setInspectionByVersion(nextInspectionByVersion);
      setArtifactInspectionSummary(payload);
      if ((payload.changed_versions || 0) > 0) {
        message.warning(t('发现 {count} 个版本与远端不一致', { count: payload.changed_versions }));
      } else {
        message.success(response?.data?.message || t('批量检查成功'));
      }
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('批量检查失败')));
    } finally {
      setCheckingArtifact(false);
    }
  };

  const handleCheckOrigin = async (record) => {
    if (!selectedArtifact) {
      return;
    }
    const actionKey = `${selectedArtifact.kind}:${selectedArtifact.name}:${record.version}`;
    setCheckingVersion(actionKey);
    try {
      const response = await api.checkArtifactOrigin(selectedArtifact.kind, selectedArtifact.name, record.version, {
        token: remoteToken,
      });
      const result = response?.data?.data?.result || null;
      if (result) {
        setInspectionByVersion((current) => ({
          ...current,
          [record.version]: result,
        }));
      }
      if (result?.changed) {
        message.warning(t('版本 {version} 与远端不一致', { version: record.version }));
      } else {
        message.success(response?.data?.message || t('远端检查成功'));
      }
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('远端检查失败')));
    } finally {
      setCheckingVersion('');
    }
  };

  const handleResyncVersion = async (record) => {
    if (!selectedArtifact) {
      return;
    }
    const actionKey = `${selectedArtifact.kind}:${selectedArtifact.name}:${record.version}`;
    setResyncingVersion(actionKey);
    try {
      const response = await api.resyncArtifactVersion(selectedArtifact.kind, selectedArtifact.name, record.version, {
        token: remoteToken,
      });
      message.success(response?.data?.message || t('重新同步成功'));
      const [artifact] = await Promise.all([
        loadArtifactVersions(selectedArtifact.kind, selectedArtifact.name),
        loadArtifacts(),
      ]);
      setSelectedArtifact((current) => (current ? { kind: current.kind, name: current.name, ...artifact } : current));
      setVersions(artifact.releases || []);
      setArtifactInspectionSummary(null);
      setInspectionByVersion((current) => {
        const next = { ...current };
        delete next[record.version];
        return next;
      });
      setVersionDetails((current) => {
        const next = { ...current };
        delete next[record.version];
        return next;
      });
      setExpandedRowKeys((current) => current.filter((item) => item !== record.version));
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('重新同步失败')));
    } finally {
      setResyncingVersion('');
    }
  };

  const handleDeleteArtifact = async (record) => {
    const actionKey = `${record.kind}:${record.name}`;
    setDeletingArtifactKey(actionKey);
    try {
      await api.deleteArtifact(record.kind, record.name);
      if (selectedArtifact?.kind === record.kind && selectedArtifact?.name === record.name) {
        resetModalState();
      }
      await loadArtifacts();
      message.success(t('制品已删除'));
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('删除制品失败')));
    } finally {
      setDeletingArtifactKey('');
    }
  };

  const handleDeleteVersion = async (record) => {
    if (!selectedArtifact) {
      return;
    }
    const actionKey = `${selectedArtifact.kind}:${selectedArtifact.name}:${record.version}`;
    setDeletingVersionKey(actionKey);
    try {
      const response = await api.deleteArtifactVersion(selectedArtifact.kind, selectedArtifact.name, record.version);
      const result = response?.data?.data?.result || {};
      await loadArtifacts();
      if (result?.artifact_deleted) {
        resetModalState();
        message.success(t('最后一个版本已删除，制品已一并移除'));
        return;
      }

      const artifact = await loadArtifactVersions(selectedArtifact.kind, selectedArtifact.name);
      setSelectedArtifact((current) => (current ? { kind: current.kind, name: current.name, ...artifact } : current));
      setVersions(artifact.releases || []);
      setArtifactInspectionSummary(null);
      setInspectionByVersion((current) => {
        const next = { ...current };
        delete next[record.version];
        return next;
      });
      setVersionDetails((current) => {
        const next = { ...current };
        delete next[record.version];
        return next;
      });
      setExpandedRowKeys((current) => current.filter((item) => item !== record.version));
      message.success(t('版本已删除'));
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('删除版本失败')));
    } finally {
      setDeletingVersionKey('');
    }
  };

  const filteredArtifacts = artifacts.filter((item) => {
    const provider = getTransportProvider(item) || 'local';
    if (kindFilter !== 'all' && item.kind !== kindFilter) {
      return false;
    }
    if (providerFilter !== 'all' && provider !== providerFilter) {
      return false;
    }
    return matchesArtifactSearch(item, query);
  });

  const kindOptions = Array.from(new Set(artifacts.map((item) => item.kind).filter(Boolean))).sort();
  const providerOptions = Array.from(new Set(artifacts.map((item) => getTransportProvider(item) || 'local').filter(Boolean))).sort();
  const hasGitHubVersions = versions.some((item) => getTransportProvider(item) === 'github_release');
  const batchInspectionAlert = inspectionAlertProps(artifactInspectionSummary, t);
  const githubArtifactCount = artifacts.filter((item) => getTransportProvider(item) === 'github_release').length;

  const columns = [
    { title: t('名称'), dataIndex: 'name', key: 'name' },
    { title: t('类型'), dataIndex: 'kind', key: 'kind', render: (kind) => <Tag>{kind}</Tag> },
    {
      title: t('来源'),
      key: 'provider',
      render: (_, record) => renderTransportTag(getTransportProvider(record) || 'local', t),
    },
    {
      title: t('模式'),
      key: 'mode',
      render: (_, record) => renderModeTag(getTransportMode(record), t),
    },
    { title: t('最新版本'), dataIndex: 'latest_version', key: 'latest_version' },
    { title: t('标题'), dataIndex: 'title', key: 'title' },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record) => (
        <Space size={4} wrap>
          <Button type="link" onClick={() => viewVersions(record)}>{t('查看版本')}</Button>
          <Popconfirm
            title={t('删除该制品？')}
            description={t('会删除该制品的所有版本、来源元数据和索引记录。')}
            onConfirm={() => handleDeleteArtifact(record)}
            okButtonProps={{ danger: true, loading: deletingArtifactKey === `${record.kind}:${record.name}` }}
          >
            <Button type="link" danger loading={deletingArtifactKey === `${record.kind}:${record.name}`}>
              {t('删除制品')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const versionColumns = [
    { title: t('版本'), dataIndex: 'version', key: 'version' },
    {
      title: t('渠道'),
      key: 'channel',
      render: (_, record) => renderChannelTags(record),
    },
    {
      title: t('发布时间'),
      dataIndex: 'published_at',
      key: 'published_at',
      render: (value) => formatDateTime(value, locale),
    },
    {
      title: t('大小'),
      dataIndex: 'size',
      key: 'size',
      render: (value) => formatBytes(value),
    },
    {
      title: t('来源'),
      key: 'transport',
      render: (_, record) => renderTransportTag(getTransportProvider(record) || 'local', t),
    },
    {
      title: t('模式'),
      key: 'mode',
      render: (_, record) => renderModeTag(getTransportMode(record), t),
    },
    {
      title: t('同步时间'),
      key: 'synced_at',
      render: (_, record) => formatDateTime(getSyncTimestamp(record), locale),
    },
    {
      title: t('SHA256'),
      dataIndex: 'sha256',
      key: 'sha256',
      render: (value) => renderCopyableText(value),
    },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record) => {
        if (!selectedArtifact) {
          return '-';
        }
        const provider = getTransportProvider(record);
        const actionKey = `${selectedArtifact.kind}:${selectedArtifact.name}:${record.version}`;
        const deleteAction = (
          <Popconfirm
            title={t('删除版本 {version}？', { version: record.version })}
            description={t('会删除该版本的包文件、manifest、origin 元数据，并更新制品索引。')}
            onConfirm={() => handleDeleteVersion(record)}
            okButtonProps={{ danger: true, loading: deletingVersionKey === actionKey }}
          >
            <Button type="link" danger loading={deletingVersionKey === actionKey}>
              {t('删除版本')}
            </Button>
          </Popconfirm>
        );
        if (provider !== 'github_release') {
          return deleteAction;
        }
        return (
          <Space size={4} wrap>
            <Button type="link" loading={checkingVersion === actionKey} onClick={() => handleCheckOrigin(record)}>
              {t('检查远端')}
            </Button>
            <Popconfirm
              title={t('重新同步该版本？')}
              description={t('会重新拉取该 GitHub Release 资产并覆盖当前 canonical storage。')}
              onConfirm={() => handleResyncVersion(record)}
              okButtonProps={{ loading: resyncingVersion === actionKey }}
            >
              <Button type="link" loading={resyncingVersion === actionKey}>
                {t('重新同步')}
              </Button>
            </Popconfirm>
            {deleteAction}
          </Space>
        );
      },
    },
  ];

  return (
    <div className="market-page">
      <PageHeader
        eyebrow={t('制品治理')}
        title={t('制品管理')}
        description={t('先看摘要，再按需展开版本详情和远端检查结果。')}
        meta={[
          { label: t('制品总数'), value: artifacts.length },
          { label: t('GitHub 来源'), value: githubArtifactCount },
          { label: t('当前筛选'), value: filteredArtifacts.length },
        ]}
      />

      <div className="market-summary-grid">
        <div className="market-summary-tile">
          <div className="market-summary-label">{t('制品总量')}</div>
          <div className="market-summary-value">{artifacts.length}</div>
          <div className="market-summary-note">{t('已收录所有已构建索引的制品')}</div>
        </div>
        <div className="market-summary-tile">
          <div className="market-summary-label">{t('当前筛选结果')}</div>
          <div className="market-summary-value">{filteredArtifacts.length}</div>
          <div className="market-summary-note">{t('搜索词、类型与来源过滤后的结果集')}</div>
        </div>
        <div className="market-summary-tile">
          <div className="market-summary-label">{t('GitHub Release 来源')}</div>
          <div className="market-summary-value">{githubArtifactCount}</div>
          <div className="market-summary-note">{t('支持远端检查和重新同步的制品')}</div>
        </div>
        <div className="market-summary-tile">
          <div className="market-summary-label">{t('本地镜像来源')}</div>
          <div className="market-summary-value">{artifacts.length - githubArtifactCount}</div>
          <div className="market-summary-note">{t('纯本地 canonical storage 托管的制品')}</div>
        </div>
      </div>

      <Card className="market-panel" title={t('制品列表')} extra={<span className="market-inline-note">{t('点击“查看版本”进入按需加载的版本历史')}</span>}>
        <Space size={12} wrap style={{ marginBottom: 16 }}>
          <Input
            allowClear
            placeholder={t('搜索名称、标题、摘要或来源')}
            style={{ width: 'min(280px, 100%)' }}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <Select value={kindFilter} onChange={setKindFilter} style={{ width: 'min(180px, 100%)' }}>
            <Select.Option value="all">{t('全部类型')}</Select.Option>
            {kindOptions.map((kind) => (
              <Select.Option key={kind} value={kind}>{kind}</Select.Option>
            ))}
          </Select>
          <Select value={providerFilter} onChange={setProviderFilter} style={{ width: 'min(180px, 100%)' }}>
            <Select.Option value="all">{t('全部来源')}</Select.Option>
            {providerOptions.map((provider) => (
              <Select.Option key={provider} value={provider}>{provider}</Select.Option>
            ))}
          </Select>
        </Space>
        <Table
          className="market-table"
          dataSource={filteredArtifacts}
          columns={columns}
          loading={loading}
          rowKey={(record) => `${record.kind}-${record.name}`}
          locale={{ emptyText: <div className="market-empty">{t('没有匹配当前筛选条件的制品')}</div> }}
        />
      </Card>

      <Modal
        title={t('版本历史 - {name}', { name: selectedArtifact?.name || '' })}
        open={modalVisible}
        onCancel={resetModalState}
        footer={null}
        width="min(920px, calc(100vw - 32px))"
      >
        {selectedArtifact && (
          <div className="market-modal-summary" style={{ marginBottom: 16 }}>
            <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
              <Descriptions.Item label={t('类型')}>{selectedArtifact.kind}</Descriptions.Item>
              <Descriptions.Item label={t('最新版本')}>{selectedArtifact.latest_version}</Descriptions.Item>
              <Descriptions.Item label={t('版本数')}>{versionsLoading ? <Spin size="small" /> : versions.length}</Descriptions.Item>
              <Descriptions.Item label={t('最新来源')}>{renderTransportTag(getTransportProvider(selectedArtifact) || 'local', t)}</Descriptions.Item>
              <Descriptions.Item label={t('摘要')}>{selectedArtifact.summary || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('标题')} span={2}>{selectedArtifact.title}</Descriptions.Item>
            </Descriptions>
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}>
              <Popconfirm
                title={t('删除该制品？')}
                description={t('会删除该制品的所有版本、来源元数据和索引记录。')}
                onConfirm={() => handleDeleteArtifact(selectedArtifact)}
                okButtonProps={{ danger: true, loading: deletingArtifactKey === `${selectedArtifact.kind}:${selectedArtifact.name}` }}
              >
                <Button danger loading={deletingArtifactKey === `${selectedArtifact.kind}:${selectedArtifact.name}`}>
                  {t('删除制品')}
                </Button>
              </Popconfirm>
            </div>
          </div>
        )}
        {hasGitHubVersions && (
          <Space direction="vertical" size={12} style={{ width: '100%', marginBottom: 16 }}>
            <Alert
              type="info"
              showIcon
              message={t('该制品包含 GitHub Release 同步版本。可使用临时 Token 批量检查全部远端版本，或对单个版本执行远端检查与重新同步。')}
            />
            <Input.Password
              placeholder={t('GitHub Token（可选，仅用于当前弹窗内的远端检查/重新同步）')}
              value={remoteToken}
              onChange={(event) => setRemoteToken(event.target.value)}
            />
            <Space size={12} wrap>
              <Button onClick={handleCheckArtifactOrigins} loading={checkingArtifact}>
                {t('检查全部 GitHub 版本')}
              </Button>
              <Button onClick={resetInspectionState}>
                {t('清空检查结果')}
              </Button>
            </Space>
            {batchInspectionAlert && (
              <Alert
                type={batchInspectionAlert.type}
                showIcon
                message={batchInspectionAlert.message}
                description={batchInspectionAlert.description}
              />
            )}
          </Space>
        )}
        <Table
          className="market-table"
          dataSource={versions}
          columns={versionColumns}
          loading={versionsLoading}
          rowKey="version"
          pagination={false}
          size="small"
          scroll={{ x: 1120 }}
          locale={{ emptyText: versionsLoading ? t('加载版本中...') : t('暂无版本数据') }}
          expandable={{
            expandedRowKeys,
            onExpand: handleVersionExpand,
            rowExpandable: (record) => Boolean(record?.version),
            expandedRowRender: (record) => {
              const detail = versionDetails[record.version];
              const detailLoading = Boolean(versionDetailLoading[record.version]);
              const inspection = inspectionByVersion[record.version];

              if (!detail && detailLoading) {
                return (
                  <Space size={8} style={{ width: '100%', padding: '8px 0' }}>
                    <Spin size="small" />
                    <Typography.Text type="secondary">{t('加载版本详情中...')}</Typography.Text>
                  </Space>
                );
              }

              if (!detail) {
                return (
                  <Alert
                    type="info"
                    showIcon
                    message={t('版本详情尚未加载')}
                    description={t('展开后会按需请求该版本的来源、定位器与缓存信息。')}
                  />
                );
              }

              const locator = detail.origin?.locator || {};
              const sync = detail.origin?.sync || detail.sync || {};
              const cache = detail.origin?.cache || {};
              const detailRecord = detail || record;

              return (
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  {inspection && (
                    <Alert
                      type={inspection.changed ? 'warning' : 'success'}
                      showIcon
                      message={inspection.changed ? t('远端制品与当前缓存不一致') : t('远端制品与当前缓存一致')}
                      description={inspection.changed
                        ? t('变更字段: {fields}', { fields: (inspection.changed_fields || []).join(', ') || 'unknown' })
                        : t('最近检查的远端 SHA256: {sha}', { sha: inspection.sha256 })}
                    />
                  )}
                  <Descriptions size="small" column={DESCRIPTION_COLUMNS}>
                    <Descriptions.Item label={t('标题')}>{detailRecord.title || record.title || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('摘要')}>{detailRecord.summary || record.summary || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('来源 Provider')}>
                      {renderTransportTag(getTransportProvider(detailRecord) || getTransportProvider(record) || 'local', t)}
                    </Descriptions.Item>
                    <Descriptions.Item label={t('传输模式')}>
                      {renderModeTag(getTransportMode(detailRecord) || getTransportMode(record), t)}
                    </Descriptions.Item>
                    <Descriptions.Item label={t('下载地址')} span={2}>
                      {renderCopyableText(detailRecord.download_url || record.download_url)}
                    </Descriptions.Item>
                    <Descriptions.Item label={t('所有者 / 仓库')}>
                      {locator.owner && locator.repo ? `${locator.owner}/${locator.repo}` : '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label={t('标签')}>{locator.tag || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('资产')}>{renderCopyableText(locator.asset_name)}</Descriptions.Item>
                    <Descriptions.Item label={t('同步策略')}>{sync.strategy || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('最近同步')}>{formatDateTime(sync.last_synced_at, locale)}</Descriptions.Item>
                    <Descriptions.Item label={t('缓存路径')} span={2}>{renderCopyableText(cache.artifact_path)}</Descriptions.Item>
                    <Descriptions.Item label={t('浏览下载地址')} span={2}>
                      {locator.browser_download_url ? (
                        <Typography.Link href={locator.browser_download_url} target="_blank" rel="noreferrer">
                          {locator.browser_download_url}
                        </Typography.Link>
                      ) : '-'}
                    </Descriptions.Item>
                    {inspection && (
                      <>
                        <Descriptions.Item label={t('远端 SHA256')} span={2}>{renderCopyableText(inspection.sha256)}</Descriptions.Item>
                        <Descriptions.Item label={t('远端大小')}>{formatBytes(inspection.asset_size)}</Descriptions.Item>
                        <Descriptions.Item label={t('远端更新时间')}>{formatDateTime(inspection.asset_updated_at, locale)}</Descriptions.Item>
                      </>
                    )}
                  </Descriptions>
                </Space>
              );
            },
          }}
        />
      </Modal>
    </div>
  );
}
