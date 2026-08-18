import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

import { apiUrl, isSetupRequired } from "@/lib/api"

const PUBLIC_PATHS = ["/login", "/setup", "/invite"]
const PENDING_ALLOWED_PATHS = ["/awaiting-approval"]
const SESSION_COOKIE = "factorymate_session"

type SessionLookup =
  | { kind: "none" }
  | { kind: "ok"; status: string }
  | { kind: "unavailable" }

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`)
  )
}

function isPendingAllowedPath(pathname: string): boolean {
  return PENDING_ALLOWED_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`)
  )
}

async function lookupSessionUser(request: NextRequest): Promise<SessionLookup> {
  const session = request.cookies.get(SESSION_COOKIE)?.value
  if (!session) {
    return { kind: "none" }
  }

  try {
    const response = await fetch(apiUrl("/auth/me", { server: true }), {
      headers: {
        Cookie: `${SESSION_COOKIE}=${session}`,
      },
      cache: "no-store",
    })
    if (response.status === 401 || response.status === 403) {
      return { kind: "none" }
    }
    if (!response.ok) {
      return { kind: "unavailable" }
    }
    const body = (await response.json()) as { status?: string }
    return { kind: "ok", status: body.status ?? "" }
  } catch {
    return { kind: "unavailable" }
  }
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
      const lookup = await lookupSessionUser(request)
      if (lookup.kind === "unavailable") {
        return new NextResponse("Service unavailable", { status: 503 })
      }
      if (lookup.kind === "none") {
        return NextResponse.next()
      }
      if (lookup.kind === "ok" && lookup.status === "pending_approval") {
        return NextResponse.redirect(new URL("/awaiting-approval", request.url))
      }
      return NextResponse.redirect(new URL("/", request.url))
    }

    return NextResponse.next()
  }

  if (!hasSession) {
    const destination = setupRequired ? "/setup" : "/login"
    return NextResponse.redirect(new URL(destination, request.url))
  }

  const lookup = await lookupSessionUser(request)
  if (lookup.kind === "unavailable") {
    return new NextResponse("Service unavailable", { status: 503 })
  }
  if (lookup.kind === "none") {
    return NextResponse.redirect(new URL("/login", request.url))
  }
  if (lookup.kind === "ok" && lookup.status === "pending_approval" && !isPendingAllowedPath(pathname)) {
    return NextResponse.redirect(new URL("/awaiting-approval", request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
}
