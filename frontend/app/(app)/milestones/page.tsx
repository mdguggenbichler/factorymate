import { MilestonesView } from "@/components/milestones/milestones-view"
import { serverApiFetch } from "@/lib/api-server"
import type { MilestonesResponse } from "@/lib/api-types"

export default async function MilestonesPage() {
  const data = await serverApiFetch<MilestonesResponse>("/milestones")

  return <MilestonesView groups={data.groups} />
}
