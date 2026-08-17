import { PlayersContent } from "@/components/players/players-content"
import { serverApiFetch } from "@/lib/api-server"
import type {
  PaginatedResponse,
  PlayerHistoryEvent,
  PlayersResponse,
} from "@/lib/api-types"

export default async function PlayersPage() {
  const [playersData, historyData] = await Promise.all([
    serverApiFetch<PlayersResponse>("/players"),
    serverApiFetch<PaginatedResponse<PlayerHistoryEvent>>(
      "/players/history?limit=50"
    ),
  ])

  return (
    <PlayersContent
      players={playersData.players}
      history={historyData.items}
    />
  )
}
