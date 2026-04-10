import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import './globals.css'
import { Providers } from './providers'
import { Toaster } from 'react-hot-toast'

const APP_NAME = 'AuraLogic'
const APP_DESCRIPTION = 'Self-hosted commerce platform for physical and virtual products.'
const inter = Inter({ subsets: ['latin'] })

function resolveMetadataBase(): URL | undefined {
  const appURL = String(process.env.NEXT_PUBLIC_APP_URL || '')
    .trim()
    .replace(/\/+$/g, '')
  if (!appURL) {
    return undefined
  }
  try {
    return new URL(appURL)
  } catch {
    return undefined
  }
}

export const metadata: Metadata = {
  metadataBase: resolveMetadataBase(),
  applicationName: APP_NAME,
  title: {
    default: APP_NAME,
    template: `%s | ${APP_NAME}`,
  },
  description: APP_DESCRIPTION,
  openGraph: {
    type: 'website',
    siteName: APP_NAME,
    title: APP_NAME,
    description: APP_DESCRIPTION,
  },
  twitter: {
    card: 'summary_large_image',
    title: APP_NAME,
    description: APP_DESCRIPTION,
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  // 防止白屏闪烁的脚本
  const themeScript = `
    (function() {
      try {
        var theme = localStorage.getItem('auralogic-theme') || 'system';
        var isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
        document.documentElement.classList.add(isDark ? 'dark' : 'light');
      } catch (e) {}
    })();
  `;

  // 防止语言属性闪烁的脚本
  const localeScript = `
    (function() {
      try {
        var locale = localStorage.getItem('auralogic_locale');
        if (!locale) {
          locale = navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
        }
        document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en';
        window.__LOCALE__ = locale;
      } catch (e) {}
    })();
  `;

  // 预加载 app_name 并设置 document.title + 预加载主题色避免蓝色闪烁
  const appNameScript = `
    (function() {
      try {
        var n = localStorage.getItem('auralogic_app_name') || '${APP_NAME}';
        window.__APP_NAME__ = n;
        var currentTitle = (document.title || '').trim();
        if (!currentTitle || currentTitle === '${APP_NAME}') {
          document.title = n;
        }
      } catch (e) {
        window.__APP_NAME__ = '${APP_NAME}';
        if (!document.title) document.title = '${APP_NAME}';
      }
      try {
        var pc = localStorage.getItem('auralogic_primary_color');
        if (pc) {
          document.documentElement.style.setProperty('--primary', pc);
          document.documentElement.style.setProperty('--ring', pc);
        }
        var bc = localStorage.getItem('auth_branding_cache');
        if (bc) window.__AUTH_BRAND__ = JSON.parse(bc);
      } catch (e) {}
    })();
  `;

  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="icon" href="/favicon.ico" sizes="any" />
        <script dangerouslySetInnerHTML={{ __html: themeScript + localeScript + appNameScript }} />
      </head>
      <body className={inter.className}>
        <Providers>
          {children}
          <Toaster
            position="top-right"
            toastOptions={{
              // 默认样式 - 自动适配暗色模式
              className: '',
              style: {
                background: 'hsl(var(--card))',
                color: 'hsl(var(--card-foreground))',
                border: '1px solid hsl(var(--border))',
              },
              // 成功提示
              success: {
                iconTheme: {
                  primary: 'hsl(142.1 76.2% 36.3%)',
                  secondary: 'white',
                },
              },
              // 错误提示
              error: {
                iconTheme: {
                  primary: 'hsl(0 84.2% 60.2%)',
                  secondary: 'white',
                },
              },
            }}
          />
        </Providers>
      </body>
    </html>
  )
}

