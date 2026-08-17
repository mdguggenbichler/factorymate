import { getTranslations } from "next-intl/server"

import { PagePlaceholder } from "@/components/page-placeholder"

export default async function NotificationLogPage() {
  const t = await getTranslations("nav")
  return <PagePlaceholder title={t("settingsNotificationLog")} />
}
