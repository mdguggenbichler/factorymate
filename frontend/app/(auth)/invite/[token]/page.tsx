import { InviteAcceptForm } from "@/components/invite-accept-form"
import { apiFetch, ApiError } from "@/lib/api"
import type { InvitePreview } from "@/lib/auth-client"

type InvitePageProps = {
  params: Promise<{ token: string }>
}

async function loadInvite(token: string): Promise<
  | { ok: true; invite: InvitePreview }
  | { ok: false; errorStatus: number }
> {
  try {
    const invite = await apiFetch<InvitePreview>(`/invites/${token}`, {
      server: true,
    })
    return { ok: true, invite }
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500
    return { ok: false, errorStatus: status }
  }
}

export default async function InvitePage({ params }: InvitePageProps) {
  const { token } = await params
  const result = await loadInvite(token)

  if (!result.ok) {
    return <InviteAcceptForm token={token} errorStatus={result.errorStatus} />
  }

  return (
    <InviteAcceptForm
      token={token}
      role={result.invite.role}
      expiresAt={result.invite.expiresAt}
    />
  )
}
