import type { NextConfig } from "next"
import createNextIntlPlugin from "next-intl/plugin"
import { existsSync, readFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const withNextIntl = createNextIntlPlugin("./i18n/request.ts")

const backendUrl = process.env.BACKEND_URL ?? "http://localhost:8080"
const frontendRoot = path.dirname(fileURLToPath(import.meta.url))

function readAppVersion(): string {
  const candidates = [
    path.join(frontendRoot, "../VERSION"),
    "/VERSION",
  ]
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return readFileSync(candidate, "utf8").trim()
    }
  }
  return "dev"
}

const nextConfig: NextConfig = {
  output: "standalone",
  env: {
    NEXT_PUBLIC_APP_VERSION: readAppVersion(),
  },
  turbopack: {
    root: frontendRoot,
  },
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${backendUrl}/api/:path*`,
      },
      {
        source: "/healthz",
        destination: `${backendUrl}/healthz`,
      },
    ]
  },
}

export default withNextIntl(nextConfig)
