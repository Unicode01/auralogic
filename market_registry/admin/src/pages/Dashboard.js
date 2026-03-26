import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Card, Descriptions, Table, Tag, Typography, message } from 'antd';
import ReactECharts from 'echarts-for-react';
import { api, getAPIErrorMessage } from '../api/client';
import PageHeader from '../components/PageHeader';
import RegistryStatusCard from '../components/RegistryStatusCard';
import { useI18n } from '../i18n';
import { useThemeMode } from '../theme';

function formatChangeLabel(value, t) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric === 0) {
    return t('本期无明显波动');
  }
  const prefix = numeric > 0 ? '+' : '-';
  return `${prefix}${Math.abs(numeric).toFixed(1)}% ${t('较上期')}`;
}

function formatBytes(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return '0 B';
  }
  if (numeric < 1024) {
    return `${numeric} B`;
  }
  if (numeric < 1024 * 1024) {
    return `${(numeric / 1024).toFixed(1)} KB`;
  }
  if (numeric < 1024 * 1024 * 1024) {
    return `${(numeric / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(numeric / (1024 * 1024 * 1024)).toFixed(1)} GB`;
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

export default function Dashboard() {
  const [stats, setStats] = useState({});
  const [registryStatus, setRegistryStatus] = useState(null);
  const [loading, setLoading] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const { t } = useI18n();
  const { themeMode } = useThemeMode();
  const isDark = themeMode === 'dark';

  const loadData = useCallback(async (activeGuard = () => true) => {
    setLoading(true);
    try {
      const [statsRes, registryRes] = await Promise.all([
        api.getStats(),
        api.getRegistryStatus(),
      ]);
      if (!activeGuard()) {
        return;
      }
      setStats(statsRes.data.data || {});
      setRegistryStatus(registryRes.data.data || null);
    } catch (error) {
      if (activeGuard()) {
        message.error(getAPIErrorMessage(error, t('加载仪表盘数据失败')));
      }
    } finally {
      if (activeGuard()) {
        setLoading(false);
      }
    }
  }, [t]);

  useEffect(() => {
    let active = true;
    loadData(() => active);
    return () => {
      active = false;
    };
  }, [loadData]);

  const handleReindex = useCallback(async () => {
    setReindexing(true);
    try {
      const res = await api.rebuildRegistry();
      message.success(res?.data?.message || t('重建成功'));
      setRegistryStatus(res?.data?.data?.status || null);
      await loadData(() => true);
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('重建快照失败')));
    } finally {
      setReindexing(false);
    }
  }, [loadData, t]);

  const chartOption = useMemo(() => ({
    grid: {
      left: 28,
      right: 20,
      top: 24,
      bottom: 24,
      containLabel: true,
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? 'rgba(7, 14, 24, 0.96)' : 'rgba(18, 38, 63, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#f4f8ff' },
    },
    xAxis: {
      type: 'category',
      data: stats.dates || [],
      axisLine: { lineStyle: { color: isDark ? 'rgba(133, 160, 201, 0.28)' : '#c9d6e4' } },
      axisLabel: { color: isDark ? '#91a7c3' : '#60758d' },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: isDark ? 'rgba(133, 160, 201, 0.14)' : 'rgba(120, 145, 170, 0.14)' } },
      axisLabel: { color: isDark ? '#91a7c3' : '#60758d' },
    },
    series: [{
      data: stats.downloads || [],
      type: 'line',
      smooth: true,
      symbolSize: 8,
      lineStyle: { width: 3, color: '#1558d6' },
      itemStyle: { color: '#1558d6' },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(21, 88, 214, 0.28)' },
            { offset: 1, color: isDark ? 'rgba(21, 88, 214, 0.08)' : 'rgba(21, 88, 214, 0.03)' },
          ],
        },
      },
    }],
  }), [isDark, stats.dates, stats.downloads]);

  const summaryCards = useMemo(() => ([
    {
      key: 'artifacts',
      label: t('总制品数'),
      value: stats.totalArtifacts || 0,
      note: `${stats.popular?.length || 0} ${t('个热门制品进入榜单')}`,
    },
    {
      key: 'downloads',
      label: t('总下载量'),
      value: stats.totalDownloads || 0,
      note: formatChangeLabel(stats.downloadGrowth, t),
    },
    {
      key: 'visits',
      label: t('今日访问'),
      value: stats.todayVisits || 0,
      note: `${stats.dates?.length || 0} ${t('天趋势可用')}`,
    },
    {
      key: 'health',
      label: t('快照健康度'),
      value: formatRegistryStatus(registryStatus?.status || 'idle', t),
      note: registryStatus?.message || t('尚未完成快照检查'),
    },
  ]), [stats.totalArtifacts, stats.popular, stats.totalDownloads, stats.downloadGrowth, stats.todayVisits, stats.dates, registryStatus, t]);
  const storage = stats.storage || {};
  const storageNote = storage.available
    ? `${storage.fileCount || 0} ${t('个对象 / 文件')}`
    : (storage.error || t('暂未暴露存储摘要'));

  return (
    <div className="market-page">
      <PageHeader
        eyebrow={t('控制台概览')}
        title={t('仪表盘')}
        description={t('查看快照健康度、下载趋势、热门制品和当前存储占用。')}
        meta={[
          { label: t('制品总量'), value: stats.totalArtifacts || 0 },
          { label: t('热门条目'), value: stats.popular?.length || 0 },
          { label: t('快照状态'), value: formatRegistryStatus(registryStatus?.status || 'idle', t) },
          { label: t('存储后端'), value: storage.displayName || storage.backend || t('未知') },
          { label: t('已用空间'), value: formatBytes(storage.totalBytes) },
        ]}
      />

      <div className="market-grid market-grid-4">
        {summaryCards.map((item) => (
          <Card key={item.key} className="market-panel market-stat-card" loading={loading}>
            <div className="market-stat-label">{item.label}</div>
            <div className="market-stat-value">{item.value}</div>
            <div className="market-stat-note">{item.note}</div>
          </Card>
        ))}
      </div>

      <div className="market-grid market-grid-3">
        <Card
          className="market-panel"
          title={t('下载趋势')}
          extra={<span className="market-inline-note">{t('最近 {count} 个统计点', { count: stats.dates?.length || 0 })}</span>}
          loading={loading}
        >
          <ReactECharts option={chartOption} style={{ height: 320 }} />
        </Card>

        <RegistryStatusCard
          title={t('Registry 快照状态')}
          status={registryStatus}
          loading={loading}
          reindexing={reindexing}
          onReindex={handleReindex}
        />

        <Card
          className="market-panel"
          title={t('存储概览')}
          extra={<span className="market-inline-note">{storageNote}</span>}
          loading={loading}
        >
          {storage.available ? (
            <Descriptions size="small" column={1}>
              <Descriptions.Item label={t('存储类型')}>{storage.displayName || storage.backend || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('已用空间')}>{formatBytes(storage.totalBytes)}</Descriptions.Item>
              <Descriptions.Item label={t('对象 / 文件')}>{storage.fileCount || 0}</Descriptions.Item>
              <Descriptions.Item label={t('位置')}>
                {storage.location ? (
                  <Typography.Text copyable ellipsis={{ tooltip: storage.location }} style={{ maxWidth: '100%' }}>
                    {storage.location}
                  </Typography.Text>
                ) : (
                  '-'
                )}
              </Descriptions.Item>
            </Descriptions>
          ) : (
            <Alert
              type="info"
              showIcon
              message={t('存储摘要暂不可用')}
              description={storage.error || t('当前存储后端未提供使用量统计。')}
            />
          )}
        </Card>
      </div>

      <Card
        className="market-panel"
        title={t('热门制品')}
        extra={<span className="market-inline-note">{t('按下载量聚合的近期热门条目')}</span>}
        loading={loading}
      >
        <Table
          className="market-table"
          dataSource={stats.popular || []}
          pagination={false}
          rowKey={(record) => `${record.kind}-${record.name}`}
          locale={{ emptyText: <div className="market-empty">{t('暂时没有热门制品数据')}</div> }}
          columns={[
            {
              title: t('制品名称'),
              dataIndex: 'name',
              key: 'name',
              render: (value) => <strong>{value}</strong>,
            },
            {
              title: t('类型'),
              dataIndex: 'kind',
              key: 'kind',
              render: (value) => <Tag>{value}</Tag>,
            },
            {
              title: t('下载量'),
              dataIndex: 'downloads',
              key: 'downloads',
            },
          ]}
        />
      </Card>
    </div>
  );
}
