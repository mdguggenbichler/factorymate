import { notFound } from "next/navigation"

import { PlannerEditor } from "@/components/planner/planner-editor"
import { ApiError } from "@/lib/api"
import { serverApiFetch } from "@/lib/api-server"
import type { PlannerCatalog } from "@/lib/planner/catalog-types"
import type { PlannerPlanDetail } from "@/lib/api-types"

type PageProps = {
  params: Promise<{ id: string }>
}

export default async function PlannerEditorPage({ params }: PageProps) {
  const { id } = await params
  const planId = Number(id)
  if (!Number.isFinite(planId)) {
    notFound()
  }

  let plan: PlannerPlanDetail
  let catalog: PlannerCatalog

  try {
    ;[plan, catalog] = await Promise.all([
      serverApiFetch<PlannerPlanDetail>(`/planner/plans/${planId}`),
      serverApiFetch<PlannerCatalog>("/planner/catalog"),
    ])
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound()
    }
    throw err
  }

  return <PlannerEditor initialPlan={plan} catalog={catalog} />
}
