import { redirect } from "next/navigation"

import { AppShell } from "@/components/app-shell"
import { getCurrentUser } from "@/lib/auth-server"

export default async function AppLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const user = await getCurrentUser()

  if (!user) {
    redirect("/login")
  }

  if (user.status === "pending_approval") {
    redirect("/awaiting-approval")
  }

  return <AppShell user={user}>{children}</AppShell>
}
