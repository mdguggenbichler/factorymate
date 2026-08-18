import { redirect } from "next/navigation"

import { getCurrentUser } from "@/lib/auth-server"

export default async function AwaitingApprovalLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const user = await getCurrentUser()

  if (!user) {
    redirect("/login")
  }

  return (
    <div className="flex min-h-svh w-full flex-col">
      {children}
    </div>
  )
}
