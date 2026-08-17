import "server-only"

import { cookies } from "next/headers"

import { apiUrl } from "@/lib/api"
import type { User } from "@/lib/auth-types"

export async function getCurrentUser(): Promise<User | null> {
  const cookieStore = await cookies()
  const session = cookieStore.get("factorymate_session")

  if (!session?.value) {
    return null
  }

  try {
    const response = await fetch(apiUrl("/auth/me", { server: true }), {
      headers: {
        Cookie: `${session.name}=${session.value}`,
      },
      cache: "no-store",
    })

    if (!response.ok) {
      return null
    }

    return (await response.json()) as User
  } catch {
    return null
  }
}
