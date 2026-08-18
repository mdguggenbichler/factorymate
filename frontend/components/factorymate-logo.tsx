import { cn } from "@/lib/utils"

type FactoryMateLogoProps = {
  variant?: "onDark" | "onLight"
  showWordmark?: boolean
  iconSize?: number
  className?: string
}

export function FactoryMateLogo({
  variant = "onDark",
  showWordmark = true,
  iconSize = 32,
  className,
}: FactoryMateLogoProps) {
  const factoryColor =
    variant === "onLight"
      ? "text-foreground"
      : "text-sidebar-foreground dark:text-white"

  return (
    <div className={cn("flex items-center gap-2", className)}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/icon-no-bg.svg"
        width={iconSize}
        height={iconSize}
        alt=""
        className="shrink-0"
      />
      {showWordmark ? (
        <span
          className="truncate text-sm font-semibold leading-tight group-data-[collapsible=icon]:hidden"
          aria-hidden
        >
          <span className={factoryColor}>Factory</span>
          <span className="text-[#F2A03D]">Mate</span>
        </span>
      ) : null}
    </div>
  )
}
