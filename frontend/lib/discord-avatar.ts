/** Default Discord avatar when no custom avatar hash is stored. */
export function discordDefaultAvatarUrl(userId: string): string {
  try {
    const index = Number(BigInt(userId) >> BigInt(22)) % 6
    return `https://cdn.discordapp.com/embed/avatars/${index}.png`
  } catch {
    return "https://cdn.discordapp.com/embed/avatars/0.png"
  }
}
