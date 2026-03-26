import React from 'react';
import { Alert, Button, Card, Space, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useI18n } from '../i18n';

function renderStatusTag(status, t) {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'healthy') {
    return <Tag color="success">{t('健康')}</Tag>;
  }
  if (normalized === 'idle') {
    return <Tag>{t('未初始化')}</Tag>;
  }
  if (normalized === 'missing') {
    return <Tag color="warning">{t('缺失')}</Tag>;
  }
  if (normalized === 'stale') {
    return <Tag color="error">{t('过期')}</Tag>;
  }
  if (normalized === 'degraded') {
    return <Tag color="warning">{t('异常')}</Tag>;
  }
  return <Tag>{status || '-'}</Tag>;
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

function SnapshotTile({ label, snapshot, locale, t }) {
  return (
    <div className="market-status-tile">
      <div className="market-status-tile-title">
        <span className="market-status-tile-label">{label}</span>
        {renderStatusTag(snapshot?.status, t)}
      </div>
      <div className="market-status-path">{snapshot?.path || '-'}</div>
      <div className="market-status-meta">
        <div>{t('生成时间：{value}', { value: formatDateTime(snapshot?.generatedAt || snapshot?.generated_at, locale) })}</div>
        <div>{t('更新时间：{value}', { value: formatDateTime(snapshot?.updatedAt || snapshot?.updated_at, locale) })}</div>
        <div>{t('条目数：{value}', { value: snapshot?.itemCount ?? snapshot?.item_count ?? 0 })}</div>
      </div>
    </div>
  );
}

export default function RegistryStatusCard({
  status,
  loading = false,
  reindexing = false,
  onReindex,
  title = 'Registry 快照状态',
}) {
  const { t, locale } = useI18n();
  const issues = Array.isArray(status?.issues) ? status.issues.filter(Boolean) : [];
  const statusText = status?.message || (status?.status === 'healthy' ? t('当前快照结构健康，可以继续发布或同步。') : t('快照存在待处理问题。'));
  const issueSeparator = locale === 'zh' ? '；' : '; ';

  return (
    <Card
      title={title === 'Registry 快照状态' ? t('Registry 快照状态') : title}
      className="market-panel"
      loading={loading}
      extra={onReindex ? (
        <Button icon={<ReloadOutlined />} loading={reindexing} onClick={onReindex}>
          {t('重建快照')}
        </Button>
      ) : null}
    >
      {issues.length > 0 ? (
        <Alert
          className="market-status-banner"
          type="warning"
          showIcon
          message={statusText}
          description={issues.join(issueSeparator)}
        />
      ) : (
        <Alert
          className="market-status-banner"
          type={status?.status === 'healthy' ? 'success' : 'info'}
          showIcon
          message={statusText}
        />
      )}

      <div className="market-section-header">
        <div>
          <h3 className="market-section-title">{t('快照总览')}</h3>
          <p className="market-section-description">{t('Registry 的 Source、Catalog 与统计快照都在这里收敛展示。')}</p>
        </div>
        <Space size={10} wrap>
          <div className="market-meta-pill">
            <span className="market-meta-pill-label">{t('整体状态')}</span>
            <span>{renderStatusTag(status?.status, t)}</span>
          </div>
          <div className="market-meta-pill">
            <span className="market-meta-pill-label">{t('制品数')}</span>
            <span>{status?.artifactCount ?? 0}</span>
          </div>
          <div className="market-meta-pill">
            <span className="market-meta-pill-label">{t('检查时间')}</span>
            <span>{formatDateTime(status?.checkedAt || status?.checked_at, locale)}</span>
          </div>
        </Space>
      </div>

      <div className="market-status-grid">
        <SnapshotTile label={t('Source 快照')} snapshot={status?.source} locale={locale} t={t} />
        <SnapshotTile label={t('Catalog 快照')} snapshot={status?.catalog} locale={locale} t={t} />
        <SnapshotTile label={t('统计文件')} snapshot={status?.stats} locale={locale} t={t} />
      </div>
    </Card>
  );
}
