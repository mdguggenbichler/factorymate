import { ItemIcon } from "@/components/item-icon"
import { cn } from "@/lib/utils"

type ItemWithLabelProps = {
  className?: string | null
  label: string
  size?: number
  wrapperClassName?: string
}

export function ItemWithLabel({
  className,
  label,
  size = 24,
  wrapperClassName,
}: ItemWithLabelProps) {
  return (
    <span className={cn("inline-flex items-center gap-2", wrapperClassName)}>
      <ItemIcon className={className} size={size} />
      <span>{label}</span>
    </span>
  )
}
