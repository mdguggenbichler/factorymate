const API_PREFIX = "/api"

function normalizePath(path: string): string {
  if (path.startsWith("/api")) {
    return path
  }
  return `${API_PREFIX}${path.startsWith("/") ? path : `/${path}`}`
}

/** Backend base URL for server-side and middleware fetches. */
export function getServerApiBase(): string {
  return (
    process.env.NEXT_PUBLIC_API_URL ??
    process.env.BACKEND_URL ??
    "http://localhost:8080"
  )
}

/**
 * Resolve an API URL. Browser calls use same-origin `/api` so session cookies
 * stay on the frontend origin; server/middleware calls hit the backend directly.
 */
export function apiUrl(path: string, opts?: { server?: boolean }): string {
  const normalized = normalizePath(path)
  const useServerBase =
    opts?.server === true ||
    (opts?.server !== false && typeof window === "undefined")

  if (useServerBase) {
    return `${getServerApiBase().replace(/\/$/, "")}${normalized}`
  }

  return normalized
}

export type ApiErrorBody = {
  error?: string
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

export async function parseApiError(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as ApiErrorBody
    if (body.error) {
      return body.error
    }
  } catch {
    // ignore JSON parse failures
  }

  return response.statusText || "Request failed"
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit & { server?: boolean }
): Promise<T> {
  const { server, ...requestInit } = init ?? {}
  const headers = new Headers(requestInit.headers)
  if (requestInit.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }

  const response = await fetch(apiUrl(path, { server }), {
    ...requestInit,
    credentials: requestInit.credentials ?? "include",
    headers,
  })

  if (!response.ok) {
    throw new ApiError(response.status, await parseApiError(response))
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

/** Returns true when first-run setup is still required (no users exist). */
export async function isSetupRequired(): Promise<boolean> {
  try {
    const response = await fetch(apiUrl("/auth/setup", { server: true }), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
      cache: "no-store",
    })

    return response.status === 400
  } catch {
    return false
  }
}
