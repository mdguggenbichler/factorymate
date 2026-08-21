"use client"

import dynamic from "next/dynamic"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { AddNodePopover } from "@/components/planner/add-node-popover"
import { NodeInspector } from "@/components/planner/node-inspector"
import { PlannerCatalogProvider } from "@/components/planner/planner-catalog-context"
import { PlannerLockBanner } from "@/components/planner/planner-lock-banner"
import { PlannerToolbar } from "@/components/planner/planner-toolbar"
import { SuggestDialog } from "@/components/planner/suggest-dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { apiFetch } from "@/lib/api"
import type {
  PlannerPlanDetail,
} from "@/lib/api-types"
import { analyzeGraph } from "@/lib/planner/balance"
import type { PlannerCatalog } from "@/lib/planner/catalog-types"
import { PLANNER_GRAPH_SAVE_DEBOUNCE_MS, PLANNER_LOCK_HEARTBEAT_MS } from "@/lib/planner/constants"
import type { PlanGraph } from "@/lib/planner/graph-types"
import { applyDagreLayout } from "@/lib/planner/layout"
import { indexCatalog } from "@/lib/planner/catalog-types"

const PlannerCanvas = dynamic(
  () =>
    import("@/components/planner/canvas/planner-canvas").then(
      (m) => m.PlannerCanvas
    ),
  { ssr: false, loading: () => null }
)

type SaveStatus = "saved" | "saving" | "dirty" | "readonly" | "error"

type PlannerEditorProps = {
  initialPlan: PlannerPlanDetail
  catalog: PlannerCatalog
}

export function PlannerEditor({ initialPlan, catalog }: PlannerEditorProps) {
  const t = useTranslations("planner")
  const indexedCatalog = useMemo(() => indexCatalog(catalog), [catalog])

  const [plan, setPlan] = useState(initialPlan)
  const [graph, setGraph] = useState<PlanGraph>(initialPlan.graph)
  const [updatedAt, setUpdatedAt] = useState(initialPlan.updatedAt)
  const [saveStatus, setSaveStatus] = useState<SaveStatus>(
    initialPlan.canEdit ? "saved" : "readonly"
  )
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [suggestOpen, setSuggestOpen] = useState(false)
  const [addNodeOpen, setAddNodeOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [acquiring, setAcquiring] = useState(false)

  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const skipSave = useRef(false)

  const readOnly = !plan.canEdit || plan.status === "archived"

  const balance = useMemo(
    () => analyzeGraph(indexedCatalog, graph),
    [indexedCatalog, graph]
  )

  const refreshPlan = useCallback(async () => {
    const fresh = await apiFetch<PlannerPlanDetail>(
      `/planner/plans/${plan.id}`
    )
    setPlan(fresh)
    setUpdatedAt(fresh.updatedAt)
    skipSave.current = true
    setGraph(fresh.graph)
    setSaveStatus(fresh.canEdit ? "saved" : "readonly")
  }, [plan.id])

  const saveGraph = useCallback(
    async (nextGraph: PlanGraph, at: string) => {
      setSaveStatus("saving")
      try {
        const res = await apiFetch<{ updatedAt: string }>(
          `/planner/plans/${plan.id}/graph`,
          {
            method: "PUT",
            body: JSON.stringify({ graph: nextGraph, updatedAt: at }),
          }
        )
        setUpdatedAt(res.updatedAt)
        setSaveStatus("saved")
      } catch (err) {
        setSaveStatus("error")
        toast.error(err instanceof Error ? err.message : t("toolbar.saveError"))
      }
    },
    [plan.id, t]
  )

  const scheduleSave = useCallback(
    (nextGraph: PlanGraph) => {
      if (readOnly) return
      setSaveStatus("dirty")
      if (saveTimer.current) clearTimeout(saveTimer.current)
      saveTimer.current = setTimeout(() => {
        void saveGraph(nextGraph, updatedAt)
      }, PLANNER_GRAPH_SAVE_DEBOUNCE_MS)
    },
    [readOnly, saveGraph, updatedAt]
  )

  const handleGraphChange = useCallback(
    (nextGraph: PlanGraph) => {
      if (skipSave.current) {
        skipSave.current = false
        setGraph(nextGraph)
        return
      }
      setGraph(nextGraph)
      scheduleSave(nextGraph)
    },
    [scheduleSave]
  )

  useEffect(() => {
    if (!plan.lock.mine) return
    const tick = () => {
      if (document.hidden) return
      void apiFetch(`/planner/plans/${plan.id}/lock/heartbeat`, {
        method: "POST",
      }).catch(() => {})
    }
    const id = setInterval(tick, PLANNER_LOCK_HEARTBEAT_MS)
    return () => clearInterval(id)
  }, [plan.id, plan.lock.mine])

  useEffect(() => {
    if (!plan.lock.mine) return
    const release = () => {
      void apiFetch(`/planner/plans/${plan.id}/lock/release`, {
        method: "POST",
        keepalive: true,
      })
    }
    window.addEventListener("beforeunload", release)
    return () => window.removeEventListener("beforeunload", release)
  }, [plan.id, plan.lock.mine])

  async function acquireLock() {
    setAcquiring(true)
    try {
      await apiFetch(`/planner/plans/${plan.id}/lock`, { method: "POST" })
      await refreshPlan()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("lock.acquireFailed"))
    } finally {
      setAcquiring(false)
    }
  }

  async function releaseLock() {
    try {
      await apiFetch(`/planner/plans/${plan.id}/lock/release`, {
        method: "POST",
      })
      await refreshPlan()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("lock.releaseFailed"))
    }
  }

  async function forceRelease() {
    try {
      await apiFetch(`/planner/plans/${plan.id}/lock/force-release`, {
        method: "POST",
      })
      await refreshPlan()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("lock.forceFailed"))
    }
  }

  async function handleReset() {
    try {
      await apiFetch(`/planner/plans/${plan.id}/reset-baseline`, {
        method: "POST",
      })
      await refreshPlan()
      toast.success(t("reset.done"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("reset.failed"))
    } finally {
      setResetOpen(false)
    }
  }

  function handleLayout() {
    const laidOut = applyDagreLayout(graph)
    handleGraphChange(laidOut)
  }

  async function handleSuggestApplied(nextGraph: PlanGraph) {
    skipSave.current = true
    setGraph(nextGraph)
    await refreshPlan()
  }

  const empty = graph.nodes.length === 0

  return (
    <PlannerCatalogProvider catalog={catalog}>
      <div className="flex h-[calc(100vh-4rem)] flex-col">
        <PlannerLockBanner
          lock={plan.lock}
          canManage={plan.canManage}
          onAcquire={acquireLock}
          onRelease={releaseLock}
          onForceRelease={forceRelease}
          acquiring={acquiring}
        />
        <PlannerToolbar
          saveStatus={readOnly ? "readonly" : saveStatus}
          readOnly={readOnly}
          hasBaseline={plan.hasBaseline}
          totalPowerMW={balance.totalPowerMW}
          onSuggest={() => setSuggestOpen(true)}
          onReset={() => setResetOpen(true)}
          onLayout={handleLayout}
          onAddNode={() => setAddNodeOpen(true)}
        />
        <div className="relative min-h-0 flex-1">
          <PlannerCanvas
            catalog={indexedCatalog}
            graph={graph}
            readOnly={readOnly}
            onGraphChange={handleGraphChange}
            onNodeSelect={setSelectedNodeId}
          />
          {empty ? (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
              <div className="pointer-events-auto rounded-lg border bg-card/95 p-6 text-center shadow-lg">
                <p className="mb-4 text-muted-foreground">{t("empty.hint")}</p>
                <div className="flex justify-center gap-2">
                  <Button
                    disabled={readOnly}
                    onClick={() => setSuggestOpen(true)}
                  >
                    {t("empty.suggest")}
                  </Button>
                  <Button
                    variant="outline"
                    disabled={readOnly}
                    onClick={() => setAddNodeOpen(true)}
                  >
                    {t("empty.addNode")}
                  </Button>
                </div>
              </div>
            </div>
          ) : null}
        </div>
        <NodeInspector
          open={selectedNodeId !== null}
          onOpenChange={(open) => {
            if (!open) setSelectedNodeId(null)
          }}
          nodeId={selectedNodeId}
          graph={graph}
          readOnly={readOnly}
          onUpdate={handleGraphChange}
        />
        <SuggestDialog
          open={suggestOpen}
          onOpenChange={setSuggestOpen}
          planId={plan.id}
          onApplied={handleSuggestApplied}
        />
        <AddNodePopover
          open={addNodeOpen}
          onOpenChange={setAddNodeOpen}
          graph={graph}
          onAdd={handleGraphChange}
        />
        <AlertDialog open={resetOpen} onOpenChange={setResetOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t("reset.title")}</AlertDialogTitle>
              <AlertDialogDescription>
                {t("reset.description")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("reset.cancel")}</AlertDialogCancel>
              <AlertDialogAction onClick={handleReset}>
                {t("reset.confirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </PlannerCatalogProvider>
  )
}
