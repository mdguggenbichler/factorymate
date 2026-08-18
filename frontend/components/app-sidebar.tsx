"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"

import type { User } from "@/lib/auth-types"
import { FactoryMateLogo } from "@/components/factorymate-logo"
import { SidebarAbout } from "@/components/sidebar-about"
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
  LinkIcon,
  MessageSquareIcon,
  PackageIcon,
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
    {
      title: t("mods"),
      url: "/mods",
      icon: <PackageIcon />,
    },
    {
      title: t("connection"),
      url: "/connection",
      icon: <LinkIcon />,
    },
  ]

  const settingsItems: NavItem[] = [
    {
      title: t("settingsGeneral"),
      url: "/settings/general",
      icon: <Settings2Icon />,
    },
    {
      title: t("settingsDiscord"),
      url: "/settings/discord",
      icon: <MessageSquareIcon />,
    },
    {
      title: t("settingsConnection"),
      url: "/settings/connection",
      icon: <LinkIcon />,
    },
    {
      title: t("settingsNotificationDefaults"),
      url: "/settings/notifications/defaults",
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
              render={<Link href="/" aria-label={tCommon("appName")} />}
              className="data-[slot=sidebar-menu-button]:p-1.5!"
            >
              <FactoryMateLogo variant="onDark" />
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
        <SidebarAbout />
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
    "/mods": t("mods"),
    "/connection": t("connection"),
    "/account": t("account"),
    "/account/notifications": t("accountNotifications"),
    "/settings/general": t("settingsGeneral"),
    "/settings/discord": t("settingsDiscord"),
    "/settings/connection": t("settingsConnection"),
    "/settings/notifications/defaults": t("settingsNotificationDefaults"),
    "/settings/notifications/targets": t("settingsNotificationTargets"),
    "/settings/notifications/templates": t("settingsNotificationTemplates"),
    "/settings/notifications/log": t("settingsNotificationLog"),
    "/settings/users": t("settingsUsers"),
  }

  return titles[pathname] ?? t("overview")
}
