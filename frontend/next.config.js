/** @type {import('next').NextConfig} */
const { execSync } = require('child_process')

let gitCommit = 'dev'
try {
  gitCommit = execSync('git rev-parse --short HEAD').toString().trim()
} catch (e) {}

function extractHostname(value) {
  try {
    return new URL(String(value || '').trim()).hostname
  } catch (e) {
    return ''
  }
}

const imageDomains = new Set(['localhost', '127.0.0.1'])
for (const value of [process.env.NEXT_PUBLIC_APP_URL, process.env.NEXT_PUBLIC_API_URL]) {
  const hostname = extractHostname(value)
  if (hostname) {
    imageDomains.add(hostname)
  }
}

const nextConfig = {
  // 输出模式（standalone用于Docker部署）
  output: 'standalone',

  // 图片优化配置
  images: {
    domains: Array.from(imageDomains),
    formats: ['image/avif', 'image/webp'],
  },

  // 严格模式
  reactStrictMode: true,

  // 环境变量
  env: {
    NEXT_PUBLIC_GIT_COMMIT: process.env.NEXT_PUBLIC_GIT_COMMIT || gitCommit,
  },

  // 重定向
  async redirects() {
    return [
      {
        source: '/admin',
        destination: '/admin/dashboard',
        permanent: true,
      },
    ]
  },

  // 头部配置
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'X-Frame-Options',
            value: 'SAMEORIGIN',
          },
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'X-XSS-Protection',
            value: '1; mode=block',
          },
          {
            key: 'Referrer-Policy',
            value: 'strict-origin-when-cross-origin',
          },
          {
            key: 'Permissions-Policy',
            value: 'camera=(), microphone=(), geolocation=()',
          },
          {
            key: 'Strict-Transport-Security',
            value: 'max-age=31536000; includeSubDomains',
          },
        ],
      },
    ]
  },

  // Webpack配置（可选）
  webpack: (config, { isServer }) => {
    if (!isServer) {
      // 客户端构建优化
      config.resolve.fallback = {
        ...config.resolve.fallback,
        fs: false,
      }
    }
    return config
  },
}

module.exports = nextConfig
