function decodePathSegment(value) {
  if (typeof value !== 'string' || value === '') {
    return '';
  }
  try {
    return decodeURIComponent(value);
  } catch (error) {
    return value;
  }
}

function normalizeRawURL(rawValue) {
  if (typeof rawValue !== 'string') {
    return '';
  }
  const trimmed = rawValue.trim();
  if (!trimmed) {
    return '';
  }
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed)) {
    return trimmed;
  }
  return `https://${trimmed}`;
}

function defaultAPIBaseURL(url) {
  const protocol = url.protocol === 'http:' ? 'http:' : 'https:';
  const host = String(url.hostname || '').toLowerCase();
  if (host === 'github.com' || host === 'www.github.com') {
    return 'https://api.github.com';
  }
  if (host === 'api.github.com') {
    return `${protocol}//api.github.com`;
  }
  return `${protocol}//${url.host}/api/v3`;
}

function apiBaseURLFromAPIPath(url) {
  const protocol = url.protocol === 'http:' ? 'http:' : 'https:';
  const reposMarker = '/repos/';
  const markerIndex = url.pathname.indexOf(reposMarker);
  if (markerIndex < 0) {
    return defaultAPIBaseURL(url);
  }
  const prefix = url.pathname.slice(0, markerIndex);
  return `${protocol}//${url.host}${prefix}`;
}

export function parseGitHubReleaseURL(rawValue) {
  const normalized = normalizeRawURL(rawValue);
  if (!normalized) {
    return null;
  }

  let url;
  try {
    url = new URL(normalized);
  } catch (error) {
    return null;
  }

  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return null;
  }

  const segments = url.pathname.split('/').filter(Boolean).map((segment) => decodePathSegment(segment));
  if (segments.length < 5) {
    return null;
  }

  const reposIndex = segments.indexOf('repos');
  const releasesIndex = segments.indexOf('releases');
  const tagsIndex = segments.indexOf('tags');
  if (reposIndex >= 0 && releasesIndex === reposIndex + 3 && tagsIndex === releasesIndex + 1 && segments.length > tagsIndex + 1) {
    return {
      kind: 'release',
      owner: segments[reposIndex + 1],
      repo: segments[reposIndex + 2],
      tag: segments[tagsIndex + 1],
      asset: '',
      apiBaseURL: apiBaseURLFromAPIPath(url),
    };
  }

  const apiBaseURL = defaultAPIBaseURL(url);
  const owner = segments[0];
  const repo = segments[1];
  const scope = segments[2];
  const action = segments[3];
  if (!owner || !repo || scope !== 'releases') {
    return null;
  }

  if (action === 'tag' && segments.length >= 5 && segments[4]) {
    return {
      kind: 'release',
      owner,
      repo,
      tag: segments[4],
      asset: '',
      apiBaseURL,
    };
  }

  if (action === 'download' && segments.length >= 6 && segments[4] && segments[5]) {
    return {
      kind: 'asset',
      owner,
      repo,
      tag: segments[4],
      asset: segments.slice(5).join('/'),
      apiBaseURL,
    };
  }

  return null;
}
