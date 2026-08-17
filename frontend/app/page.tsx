import { getTranslations } from "next-intl/server"

export default async function Home() {
  const t = await getTranslations("home")

  return (
    <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-16">
      <main className="flex w-full max-w-lg flex-col gap-4 text-center">
        <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
        <p className="text-sm text-muted-foreground">{t("status")}</p>
      </main>
    </div>
  )
}
