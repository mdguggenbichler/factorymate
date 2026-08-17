"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"

import type { User } from "@/lib/auth-types"
import { NavMain } from "@/components/nav-main"
import { NavSettings } from "@/components/nav-settings"
import { NavUser } from "@/components/nav-user"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import {
  ActivityIcon,
  BotIcon,
  DogIcon,
  FactoryIcon,
  FlaskConicalIcon,
  GaugeIcon,
  LayoutDashboardIcon,
  RocketIcon,
  Settings2Icon,
  TrainFrontIcon,
  UsersIcon,
  ZapIcon,
} from "lucide-react"

type NavItem = {
  title: string
  url: string
  icon: React.ReactNode
}

export function AppSidebar({
  user,
  ...props
}: React.ComponentProps<typeof Sidebar> & { user: User }) {
  const t = useTranslations("nav")
  const tCommon = useTranslations("common")

  const viewerItems: NavItem[] = [
    {
      title: t("overview"),
      url: "/",
      icon: <LayoutDashboardIcon />,
    },
    {
      title: t("players"),
      url: "/players",
      icon: <UsersIcon />,
    },
    {
      title: t("production"),
      url: "/production",
      icon: <FactoryIcon />,
    },
    {
      title: t("power"),
      url: "/power",
      icon: <ZapIcon />,
    },
    {
      title: t("resourceSink"),
      url: "/resource-sink",
      icon: <ActivityIcon />,
    },
    {
      title: t("drones"),
      url: "/drones",
      icon: <BotIcon />,
    },
    {
      title: t("doggos"),
      url: "/doggos",
      icon: <DogIcon />,
    },
    {
      title: t("milestones"),
      url: "/milestones",
      icon: <GaugeIcon />,
    },
    {
      title: t("research"),
      url: "/research",
      icon: <FlaskConicalIcon />,
    },
    {
      title: t("vehicles"),
      url: "/vehicles",
      icon: <TrainFrontIcon />,
    },
    {
      title: t("elevator"),
      url: "/elevator",
      icon: <RocketIcon />,
    },
  ]

  const settingsItems: NavItem[] = [
    {
      title: t("settingsGeneral"),
      url: "/settings/general",
      icon: <Settings2Icon />,
    },
    {
      title: t("settingsNotificationTargets"),
      url: "/settings/notifications/targets",
      icon: <Settings2Icon />,
    },
    {
      title: t("settingsNotificationTemplates"),
      url: "/settings/notifications/templates",
      icon: <Settings2Icon />,
    },
    {
      title: t("settingsNotificationLog"),
      url: "/settings/notifications/log",
      icon: <Settings2Icon />,
    },
    {
      title: t("settingsUsers"),
      url: "/settings/users",
      icon: <UsersIcon />,
    },
  ]

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              render={<Link href="/" />}
              className="data-[slot=sidebar-menu-button]:p-1.5!"
            >
              <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <FactoryIcon className="size-4" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">{tCommon("appName")}</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={viewerItems} />
        {user.role === "admin" ? (
          <NavSettings label={t("settings")} items={settingsItems} />
        ) : null}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} />
      </SidebarFooter>
    </Sidebar>
  )
}

export function usePageTitle(): string {
  const pathname = usePathname()
  const t = useTranslations("nav")

  const titles: Record<string, string> = {
    "/": t("overview"),
    "/players": t("players"),
    "/production": t("production"),
    "/power": t("power"),
    "/resource-sink": t("resourceSink"),
    "/drones": t("drones"),
    "/doggos": t("doggos"),
    "/milestones": t("milestones"),
    "/research": t("research"),
    "/vehicles": t("vehicles"),
    "/elevator": t("elevator"),
    "/account": t("account"),
    "/settings/general": t("settingsGeneral"),
    "/settings/notifications/targets": t("settingsNotificationTargets"),
    "/settings/notifications/templates": t("settingsNotificationTemplates"),
    "/settings/notifications/log": t("settingsNotificationLog"),
    "/settings/users": t("settingsUsers"),
  }

  return titles[pathname] ?? t("overview")
}
