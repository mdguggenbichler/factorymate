import { getTranslations } from "next-intl/server"

import { PagePlaceholder } from "@/components/page-placeholder"

export default async function SettingsUsersPage() {
  const t = await getTranslations("nav")
  return <PagePlaceholder title={t("settingsUsers")} />
}
