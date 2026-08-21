import { PlannerList } from "@/components/planner/planner-list"
import { serverApiFetch } from "@/lib/api-server"
import type { PlannerPlansListResponse } from "@/lib/api-types"

export default async function PlannerPage() {
  const data = await serverApiFetch<PlannerPlansListResponse>(
    "/planner/plans"
  )

  return <PlannerList initialPlans={data.plans ?? []} />
}
