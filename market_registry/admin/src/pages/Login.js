import React, { useState } from 'react';
import { Alert, Button, Card, Form, Input, message } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api, getAPIErrorMessage } from '../api/client';
import { useI18n } from '../i18n';
import LanguageSwitcher from '../components/LanguageSwitcher';
import ThemeSwitcher from '../components/ThemeSwitcher';

export default function Login() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const { t } = useI18n();

  const onFinish = async (values) => {
    setLoading(true);
    try {
      const res = await api.login(values.username, values.password);
      localStorage.setItem('token', res.data.data.token);
      localStorage.setItem('user', JSON.stringify(res.data.data.user));
      message.success(t('登录成功'));
      navigate('/dashboard');
    } catch (error) {
      message.error(getAPIErrorMessage(error, t('登录失败')));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="market-auth-shell">
      <section className="market-auth-hero">
        <div className="market-auth-hero-inner">
          <div>
            <span className="market-auth-kicker">AuraLogic Registry Console</span>
            <h1 className="market-auth-title">{t('市场源控制台')}</h1>
            <p className="market-auth-description">
              {t('查看快照状态、发布本地制品、同步远端 Release。')}
            </p>
            <div className="market-auth-points">
              <div className="market-auth-point">
                <p className="market-auth-point-title">{t('统一入口')}</p>
                <p className="market-auth-point-copy">
                  {t('本地发布和 GitHub Release 同步都在同一个后台处理。')}
                </p>
              </div>
              <div className="market-auth-point">
                <p className="market-auth-point-title">{t('快照可见')}</p>
                <p className="market-auth-point-copy">
                  {t('Source、Catalog 和统计快照都能直接看到当前状态。')}
                </p>
              </div>
              <div className="market-auth-point">
                <p className="market-auth-point-title">{t('版本可追踪')}</p>
                <p className="market-auth-point-copy">
                  {t('每个制品都能查看来源、同步时间和版本详情。')}
                </p>
              </div>
            </div>
          </div>
          <div className="market-auth-toolbar">
            <div className="market-inline-note">
              {t('用于 market_registry 的嵌入式管理界面')}
            </div>
            <div className="market-switcher-cluster">
              <ThemeSwitcher size="small" />
              <LanguageSwitcher size="small" />
            </div>
          </div>
        </div>
      </section>

      <section className="market-auth-panel-wrap">
        <Card className="market-auth-panel" title={t('登录市场源管理后台')}>
          <p className="market-auth-subtitle">
            {t('使用管理员凭据进入 Registry 控制台，查看制品、快照和远端同步状态。')}
          </p>

          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 20 }}
            message={t('默认本地预览会使用独立数据目录，不会污染宿主主站数据。')}
          />

          <Form layout="vertical" onFinish={onFinish} initialValues={{ username: 'admin', password: 'admin' }}>
            <Form.Item name="username" label={t('用户名')} rules={[{ required: true, message: t('请输入用户名') }]}>
              <Input prefix={<UserOutlined />} placeholder={t('管理员账号')} />
            </Form.Item>
            <Form.Item name="password" label={t('密码')} rules={[{ required: true, message: t('请输入密码') }]}>
              <Input.Password prefix={<LockOutlined />} placeholder={t('管理员密码')} />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={loading} block className="market-login-button">
                {t('进入控制台')}
              </Button>
            </Form.Item>
          </Form>

          <div className="market-credentials">
            <p className="market-credentials-label">{t('当前本地预览默认凭据')}</p>
            <p className="market-credentials-copy">
              {t('用户名')}：<strong>admin</strong><br />
              {t('密码')}：<strong>admin</strong>
            </p>
          </div>
        </Card>
      </section>
    </div>
  );
}
