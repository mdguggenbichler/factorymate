import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

import { isSetupRequired } from "@/lib/api"

const PUBLIC_PATHS = ["/login", "/setup", "/invite"]
const SESSION_COOKIE = "factorymate_session"

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`)
  )
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/api") ||
    pathname === "/healthz" ||
    pathname === "/favicon.ico" ||
    /\.[^/]+$/.test(pathname)
  ) {
    return NextResponse.next()
  }

  const hasSession = Boolean(request.cookies.get(SESSION_COOKIE)?.value)
  const setupRequired = hasSession ? false : await isSetupRequired()

  if (isPublicPath(pathname)) {
    if (pathname.startsWith("/login") && setupRequired) {
      return NextResponse.redirect(new URL("/setup", request.url))
    }

    if (pathname.startsWith("/setup") && !setupRequired) {
      return NextResponse.redirect(new URL("/login", request.url))
    }

    if (pathname.startsWith("/invite")) {
      return NextResponse.next()
    }

    if (hasSession) {
      return NextResponse.redirect(new URL("/", request.url))
    }

    return NextResponse.next()
  }

  if (!hasSession) {
    const destination = setupRequired ? "/setup" : "/login"
    return NextResponse.redirect(new URL(destination, request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
}
