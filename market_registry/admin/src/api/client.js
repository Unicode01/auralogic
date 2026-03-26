import axios from 'axios';
import { getAPIBaseURL, withBasePath } from '../config/runtime';

const client = axios.create({
  baseURL: getAPIBaseURL(),
  timeout: 30000,
});

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      window.location.assign(withBasePath('/login'));
    }
    return Promise.reject(error);
  }
);

export const api = {
  login: (username, password) =>
    client.post('/admin/auth/login', { username, password }),

  publishRelease: (formData, onProgress) =>
    client.post('/admin/publish', formData, {
      onUploadProgress: (e) => onProgress?.(Math.round((e.loaded * 100) / e.total))
    }),

  previewGitHubRelease: (payload) =>
    client.post('/admin/sync/github-release/release', payload),

  syncGitHubRelease: (payload) =>
    client.post('/admin/sync/github-release', payload),

  inspectGitHubRelease: (payload) =>
    client.post('/admin/sync/github-release/inspect', payload),

  getStats: () =>
    client.get('/admin/stats/overview'),

  getRegistryStatus: () =>
    client.get('/admin/registry/status'),

  rebuildRegistry: () =>
    client.post('/admin/registry/reindex'),

  getSettings: () =>
    client.get('/admin/settings'),

  updateSettings: (payload) =>
    client.put('/admin/settings', payload),

  listArtifacts: () =>
    client.get('/admin/artifacts'),

  getArtifactVersions: (kind, name) =>
    client.get(`/admin/artifacts/${kind}/${name}`),

  getArtifactRelease: (kind, name, version) =>
    client.get(`/admin/artifacts/${kind}/${name}/${version}`),

  deleteArtifact: (kind, name) =>
    client.delete(`/admin/artifacts/${kind}/${name}`),

  deleteArtifactVersion: (kind, name, version) =>
    client.delete(`/admin/artifacts/${kind}/${name}/${version}`),

  checkArtifactOrigins: (kind, name, payload = {}) =>
    client.post(`/admin/artifacts/${kind}/${name}/check-origin`, payload),

  checkArtifactOrigin: (kind, name, version, payload = {}) =>
    client.post(`/admin/artifacts/${kind}/${name}/${version}/check-origin`, payload),

  resyncArtifactVersion: (kind, name, version, payload = {}) =>
    client.post(`/admin/artifacts/${kind}/${name}/${version}/resync`, payload),
};

export function getAPIErrorMessage(error, fallback = 'Request failed') {
  const payload = error?.response?.data;
  if (typeof payload?.error?.message === 'string' && payload.error.message.trim()) {
    return payload.error.message.trim();
  }
  if (typeof payload?.error === 'string' && payload.error.trim()) {
    return payload.error.trim();
  }
  if (typeof payload?.message === 'string' && payload.message.trim()) {
    return payload.message.trim();
  }
  if (typeof error?.message === 'string' && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

export default client;
