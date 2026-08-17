import "server-only"

import { cookies } from "next/headers"

import { apiFetch } from "@/lib/api"

async function sessionHeaders(): Promise<HeadersInit> {
  const cookieStore = await cookies()
  const session = cookieStore.get("factorymate_session")

  if (!session?.value) {
    return {}
  }

  return {
    Cookie: `${session.name}=${session.value}`,
  }
}

export async function serverApiFetch<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  const headers = new Headers(init?.headers)
  const sessionHeadersInit = await sessionHeaders()
  for (const [key, value] of Object.entries(sessionHeadersInit)) {
    if (typeof value === "string") {
      headers.set(key, value)
    }
  }

  return apiFetch<T>(path, {
    ...init,
    server: true,
    headers,
    cache: "no-store",
  })
}
