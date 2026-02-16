export interface SelectOption {
  label: string
  value: string
}

export type Locale = 'zh-CN' | 'en-US'

export interface PageProps {
  params: Record<string, string>
  searchParams: Record<string, string | string[] | undefined>
}

