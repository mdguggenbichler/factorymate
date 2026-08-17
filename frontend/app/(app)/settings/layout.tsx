import { redirect } from "next/navigation"

import { getCurrentUser } from "@/lib/auth-server"

export default async function SettingsLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const user = await getCurrentUser()

  if (!user || user.role !== "admin") {
    redirect("/")
  }

  return children
}
