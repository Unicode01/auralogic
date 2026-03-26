import { parseGitHubReleaseURL } from './githubReleaseUrl';

describe('parseGitHubReleaseURL', () => {
  test('parses a GitHub release page URL', () => {
    expect(parseGitHubReleaseURL('https://github.com/auralogic/market-packages/releases/tag/v1.2.3')).toEqual({
      kind: 'release',
      owner: 'auralogic',
      repo: 'market-packages',
      tag: 'v1.2.3',
      asset: '',
      apiBaseURL: 'https://api.github.com',
    });
  });

  test('parses an asset download URL and decodes segments', () => {
    expect(parseGitHubReleaseURL('github.com/auralogic/market-packages/releases/download/release%2F2026/plugin%20bundle.zip')).toEqual({
      kind: 'asset',
      owner: 'auralogic',
      repo: 'market-packages',
      tag: 'release/2026',
      asset: 'plugin bundle.zip',
      apiBaseURL: 'https://api.github.com',
    });
  });

  test('derives the enterprise API base URL from a browser URL', () => {
    expect(parseGitHubReleaseURL('https://git.example.com/team/repo/releases/tag/v2.0.0')).toEqual({
      kind: 'release',
      owner: 'team',
      repo: 'repo',
      tag: 'v2.0.0',
      asset: '',
      apiBaseURL: 'https://git.example.com/api/v3',
    });
  });

  test('parses a GitHub API release URL', () => {
    expect(parseGitHubReleaseURL('https://api.github.com/repos/auralogic/market-packages/releases/tags/v1.0.0')).toEqual({
      kind: 'release',
      owner: 'auralogic',
      repo: 'market-packages',
      tag: 'v1.0.0',
      asset: '',
      apiBaseURL: 'https://api.github.com',
    });
  });

  test('parses an enterprise GitHub API release URL', () => {
    expect(parseGitHubReleaseURL('https://git.example.com/api/v3/repos/team/repo/releases/tags/v3.0.0')).toEqual({
      kind: 'release',
      owner: 'team',
      repo: 'repo',
      tag: 'v3.0.0',
      asset: '',
      apiBaseURL: 'https://git.example.com/api/v3',
    });
  });

  test('returns null for unsupported URLs', () => {
    expect(parseGitHubReleaseURL('https://github.com/auralogic/market-packages/issues/1')).toBeNull();
    expect(parseGitHubReleaseURL('')).toBeNull();
  });
});
