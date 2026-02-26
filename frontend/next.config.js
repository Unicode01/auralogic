/** @type {import('next').NextConfig} */
const { execSync } = require('child_process')

let gitCommit = 'dev'
try {
    gitCommit = execSync('git rev-parse --short HEAD').toString().trim()
} catch (e) {}

const nextConfig = {
    // 输出模式（standalone用于Docker部署）
    output: 'standalone',

    // 图片优化配置
    images: {
        domains: [
            'localhost',
            'auralogic.un1c0de.com',
            'example.com',
            // 添加你的图片域名
        ],
        formats: ['image/avif', 'image/webp'],
    },

    // 严格模式
    reactStrictMode: true,

    // 环境变量
    env: {
        NEXT_PUBLIC_GIT_COMMIT: process.env.NEXT_PUBLIC_GIT_COMMIT || gitCommit,
        NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com',
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

