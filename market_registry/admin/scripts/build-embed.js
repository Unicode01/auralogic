const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const adminDir = path.resolve(__dirname, '..');
const outputDir = path.resolve(adminDir, '..', 'internal', 'adminui', 'dist');
const reactScripts = path.join(
  adminDir,
  'node_modules',
  '.bin',
  process.platform === 'win32' ? 'react-scripts.cmd' : 'react-scripts'
);
const placeholderHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <title>Market Registry Admin UI</title>
  </head>
  <body>
    <main>
      <h1>Admin UI is not built yet</h1>
      <p>Run <code>cd market_registry/admin &amp;&amp; npm install &amp;&amp; npm run build</code> to generate the embedded admin UI bundle.</p>
    </main>
  </body>
</html>
`;

fs.rmSync(outputDir, { recursive: true, force: true });
fs.mkdirSync(outputDir, { recursive: true });
fs.writeFileSync(path.join(outputDir, 'index.html'), placeholderHTML, 'utf8');

const env = {
  ...process.env,
  BUILD_PATH: outputDir,
  PUBLIC_URL: '/admin/ui',
  REACT_APP_ROUTER_BASENAME: '/admin/ui',
};

const command = process.platform === 'win32' ? 'cmd.exe' : reactScripts;
const args = process.platform === 'win32' ? ['/c', reactScripts, 'build'] : ['build'];

const result = spawnSync(command, args, {
  cwd: adminDir,
  env,
  stdio: 'inherit',
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

if (typeof result.status === 'number') {
  process.exit(result.status);
}

process.exit(1);
