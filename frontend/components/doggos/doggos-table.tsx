import { getTranslations } from "next-intl/server"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Doggo } from "@/lib/api-types"

type DoggosTableProps = {
  doggos: Doggo[]
}

function inventoryLabel(item: unknown): string {
  if (typeof item === "string") {
    return item
  }

  if (item && typeof item === "object") {
    const record = item as Record<string, unknown>
    if (typeof record.name === "string") {
      return record.name
    }
    if (typeof record.Name === "string") {
      return record.Name
    }
  }

  return "?"
}

export async function DoggosTable({ doggos }: DoggosTableProps) {
  const t = await getTranslations("doggos")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("tableTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {doggos.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.name")}</TableHead>
                  <TableHead>{t("columns.inventory")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {doggos.map((doggo) => (
                  <TableRow key={doggo.doggoId}>
                    <TableCell className="font-medium">
                      {doggo.name ?? t("unnamed")}
                    </TableCell>
                    <TableCell>
                      {doggo.inventory.length === 0 ? (
                        <span className="text-muted-foreground">{t("emptyInventory")}</span>
                      ) : (
                        <div className="flex flex-wrap gap-1">
                          {doggo.inventory.map((item, index) => (
                            <Badge key={`${doggo.doggoId}-${index}`} variant="secondary">
                              {inventoryLabel(item)}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
