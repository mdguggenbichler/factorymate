import { ResearchView } from "@/components/research/research-view"
import { serverApiFetch } from "@/lib/api-server"
import type { ResearchResponse } from "@/lib/api-types"

export default async function ResearchPage() {
  const data = await serverApiFetch<ResearchResponse>("/research")

  return <ResearchView trees={data.trees} />
}
