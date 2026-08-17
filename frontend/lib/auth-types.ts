export type UserRole = "admin" | "viewer"

export type User = {
  id: number
  username: string
  role: UserRole
}
