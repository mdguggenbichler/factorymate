import { apiFetch } from "@/lib/api"
import type { User } from "@/lib/auth-types"

export type { User, UserRole } from "@/lib/auth-types"

export async function login(username: string, password: string): Promise<User> {
  return apiFetch<User>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  })
}

export async function setupAdmin(
  username: string,
  password: string
): Promise<User> {
  return apiFetch<User>("/auth/setup", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  })
}

export async function logout(): Promise<void> {
  await apiFetch<void>("/auth/logout", {
    method: "POST",
  })
}

export async function changePassword(password: string): Promise<void> {
  await apiFetch<void>("/account/password", {
    method: "PUT",
    body: JSON.stringify({ password }),
  })
}

export type InvitePreview = {
  role: string
  expiresAt: string
  status: string
}

export async function getInvite(token: string): Promise<InvitePreview> {
  return apiFetch<InvitePreview>(`/invites/${token}`)
}

export async function acceptInvite(
  token: string,
  username: string,
  password: string
): Promise<User> {
  return apiFetch<User>(`/invites/${token}/accept`, {
    method: "POST",
    body: JSON.stringify({ username, password }),
  })
}

export type AuthConfig = {
  discordOAuthEnabled: boolean
}

export async function getAuthConfig(): Promise<AuthConfig> {
  return apiFetch<AuthConfig>("/auth/config")
}

export type RegisterCompleteResult = {
  user: User
  pendingApproval: boolean
}

export async function completeRegistration(
  token: string,
  username: string,
  pendingPlayerName: string
): Promise<RegisterCompleteResult> {
  return apiFetch<RegisterCompleteResult>("/auth/register/complete", {
    method: "POST",
    body: JSON.stringify({ token, username, pendingPlayerName }),
  })
}
