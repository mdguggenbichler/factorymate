import { getTranslations } from "next-intl/server"

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

export async function PagePlaceholder({ title }: { title: string }) {
  const t = await getTranslations("common")

  return (
    <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle>{t("comingSoonTitle")}</CardTitle>
          <CardDescription>{t("comingSoonDescription")}</CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
