export type UserRole = "admin" | "viewer"

export type User = {
  id: number
  username: string
  role: UserRole
  status?: string
  registrationSource?: string | null
  pendingPlayerName?: string | null
  externalPlatform?: string | null
  externalUserId?: string | null
  externalUsername?: string | null
  externalDisplayName?: string | null
  externalLinkedAt?: string | null
  playerId?: string | null
  playerName?: string | null
  createdAt?: string
}
