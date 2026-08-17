import { getTranslations } from "next-intl/server"

export default async function OverviewPage() {
  const t = await getTranslations("home")

  return (
    <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
        <p className="text-sm text-muted-foreground">{t("status")}</p>
      </div>
    </div>
  )
}
