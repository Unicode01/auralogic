function normalizeBasePath(value) {
  if (typeof value !== 'string') {
    return '';
  }
  const trimmed = value.trim();
  if (!trimmed || trimmed === '/') {
    return '';
  }
  const prefixed = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
  return prefixed.endsWith('/') ? prefixed.slice(0, -1) : prefixed;
}

const explicitBasePath = process.env.REACT_APP_ROUTER_BASENAME;
const publicURLBasePath = process.env.NODE_ENV === 'production' ? process.env.PUBLIC_URL : '';

export const APP_BASENAME = normalizeBasePath(explicitBasePath || publicURLBasePath);

export function withBasePath(path = '/') {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  if (!APP_BASENAME) {
    return normalizedPath;
  }
  if (normalizedPath === '/') {
    return APP_BASENAME;
  }
  return `${APP_BASENAME}${normalizedPath}`;
}

export function getAPIBaseURL() {
  if (typeof process.env.REACT_APP_API_URL === 'string' && process.env.REACT_APP_API_URL.trim()) {
    return process.env.REACT_APP_API_URL.trim();
  }
  if (process.env.NODE_ENV === 'development') {
    return 'http://localhost:18080';
  }
  return '';
}
