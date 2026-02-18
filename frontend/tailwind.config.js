/** @type {import('tailwindcss').Config} */
module.exports = {
    darkMode: ["class"],
    content: [
        './pages/**/*.{ts,tsx}',
        './components/**/*.{ts,tsx}',
        './app/**/*.{ts,tsx}',
        './src/**/*.{ts,tsx}',
    ],
    // 支付方式JS脚本动态生成的类需要加入safelist，否则会被Tailwind的tree-shaking移除
    safelist: [
        // 绿色系 (USDT等)
        'from-green-50', 'to-emerald-50', 'dark:from-green-950', 'dark:to-emerald-950',
        'border-green-200', 'dark:border-green-800',
        'text-green-700', 'dark:text-green-300', 'text-green-600', 'dark:text-green-400',
        'bg-green-100', 'dark:bg-green-900',
        // 琥珀色系 (警告提示)
        'bg-amber-50', 'dark:bg-amber-950',
        'border-amber-200', 'dark:border-amber-800',
        'text-amber-700', 'dark:text-amber-300', 'text-amber-600', 'dark:text-amber-400',
        // 黄色系 (BEP20等)
        'from-yellow-50', 'to-amber-50', 'dark:from-yellow-950', 'dark:to-amber-950',
        'border-yellow-200', 'dark:border-yellow-800',
        'text-yellow-700', 'dark:text-yellow-300', 'text-yellow-600', 'dark:text-yellow-400',
        'bg-yellow-100', 'dark:bg-yellow-900',
        // 蓝色系
        'bg-blue-50', 'dark:bg-blue-950',
        'border-blue-200', 'dark:border-blue-800',
        'text-blue-700', 'dark:text-blue-300', 'text-blue-600', 'dark:text-blue-400',
        // 灰色系 (通用暗色模式)
        'dark:bg-gray-800', 'dark:bg-gray-900',
        'dark:text-gray-100', 'dark:text-gray-200', 'dark:text-gray-300',
        'dark:border-gray-700', 'dark:border-gray-600',
        // 渐变
        'bg-gradient-to-r', 'bg-gradient-to-l', 'bg-gradient-to-b', 'bg-gradient-to-t',
        // 付款卡片动态HTML中使用的其他类
        'select-all', 'list-inside',
        'border-muted-foreground/20', 'hover:border-primary/50',
    ],
    theme: {
        container: {
            center: true,
            padding: "2rem",
            screens: {
                "2xl": "1400px",
            },
        },
        extend: {
            colors: {
                border: "hsl(var(--border))",
                input: "hsl(var(--input))",
                ring: "hsl(var(--ring))",
                background: "hsl(var(--background))",
                foreground: "hsl(var(--foreground))",
                primary: {
                    DEFAULT: "hsl(var(--primary))",
                    foreground: "hsl(var(--primary-foreground))",
                },
                secondary: {
                    DEFAULT: "hsl(var(--secondary))",
                    foreground: "hsl(var(--secondary-foreground))",
                },
                destructive: {
                    DEFAULT: "hsl(var(--destructive))",
                    foreground: "hsl(var(--destructive-foreground))",
                },
                muted: {
                    DEFAULT: "hsl(var(--muted))",
                    foreground: "hsl(var(--muted-foreground))",
                },
                accent: {
                    DEFAULT: "hsl(var(--accent))",
                    foreground: "hsl(var(--accent-foreground))",
                },
                popover: {
                    DEFAULT: "hsl(var(--popover))",
                    foreground: "hsl(var(--popover-foreground))",
                },
                card: {
                    DEFAULT: "hsl(var(--card))",
                    foreground: "hsl(var(--card-foreground))",
                },
            },
            borderRadius: {
                lg: "var(--radius)",
                md: "calc(var(--radius) - 2px)",
                sm: "calc(var(--radius) - 4px)",
            },
            keyframes: {
                "accordion-down": {
                    from: { height: 0 },
                    to: { height: "var(--radix-accordion-content-height)" },
                },
                "accordion-up": {
                    from: { height: "var(--radix-accordion-content-height)" },
                    to: { height: 0 },
                },
            },
            animation: {
                "accordion-down": "accordion-down 0.2s ease-out",
                "accordion-up": "accordion-up 0.2s ease-out",
            },
        },
    },
    plugins: [require("tailwindcss-animate")],
}

