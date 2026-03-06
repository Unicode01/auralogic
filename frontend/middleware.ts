import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // 首页统一重定向到商品页
  // 如果游客浏览关闭，/products 会在用户布局中自动跳转到 /login
  if (pathname === '/') {
    const url = request.nextUrl.clone()
    url.pathname = '/products'
    return NextResponse.redirect(url)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/'],
}
