import { BoxIcon } from "lucide-react"

import { resolveItemIconUrl } from "@/lib/item-icon"
import { cn } from "@/lib/utils"

type ItemIconProps = {
  className?: string | null
  size?: number
  alt?: string
  iconClassName?: string
}

export function ItemIcon({
  className,
  size = 24,
  alt = "",
  iconClassName,
}: ItemIconProps) {
  const url = resolveItemIconUrl(className)

  if (url) {
    return (
      <img
        src={url}
        alt={alt}
        width={size}
        height={size}
        className={cn("shrink-0 object-contain", iconClassName)}
        aria-hidden={!alt}
      />
    )
  }

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-sm bg-muted text-muted-foreground",
        iconClassName
      )}
      style={{ width: size, height: size }}
      aria-hidden
    >
      <BoxIcon style={{ width: size * 0.6, height: size * 0.6 }} />
    </span>
  )
}
