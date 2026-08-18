"use client"

import { useTranslations } from "next-intl"
import { BugIcon, ExternalLinkIcon } from "lucide-react"

import {
  APP_VERSION,
  BUG_REPORT_URL,
  GITHUB_REPO_URL,
} from "@/lib/app-meta"
import { cn } from "@/lib/utils"

type SidebarAboutProps = {
  className?: string
  variant?: "sidebar" | "auth"
}

export function SidebarAbout({
  className,
  variant = "sidebar",
}: SidebarAboutProps) {
  const t = useTranslations("about")

  const linkClassName =
    "inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"

  return (
    <div
      className={cn(
        variant === "sidebar" &&
          "px-2 py-1 group-data-[collapsible=icon]:px-0 group-data-[collapsible=icon]:py-0",
        variant === "auth" && "text-center",
        className
      )}
    >
      <p
        className={cn(
          "font-mono text-xs text-muted-foreground",
          variant === "sidebar" &&
            "mb-1.5 px-1 group-data-[collapsible=icon]:mb-0 group-data-[collapsible=icon]:text-center"
        )}
        title={t("version", { version: APP_VERSION })}
      >
        {variant === "sidebar" ? (
          <>
            <span className="group-data-[collapsible=icon]:hidden">
              {t("version", { version: APP_VERSION })}
            </span>
            <span className="hidden group-data-[collapsible=icon]:inline">
              {t("shortVersion", { version: APP_VERSION })}
            </span>
          </>
        ) : (
          t("version", { version: APP_VERSION })
        )}
      </p>
      <div
        className={cn(
          "flex flex-wrap items-center gap-x-3 gap-y-1",
          variant === "sidebar" &&
            "px-1 group-data-[collapsible=icon]:hidden",
          variant === "auth" && "justify-center"
        )}
      >
        <a
          href={GITHUB_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className={linkClassName}
        >
          <ExternalLinkIcon className="size-3.5 shrink-0" />
          {t("github")}
        </a>
        <a
          href={BUG_REPORT_URL}
          target="_blank"
          rel="noopener noreferrer"
          className={linkClassName}
        >
          <BugIcon className="size-3.5 shrink-0" />
          {t("reportBug")}
        </a>
      </div>
    </div>
  )
}
