"use client"

import { BracesIcon } from "lucide-react"
import { useTranslations } from "next-intl"

import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

type VariablePickerProps = {
  variables: string[]
  onSelect: (variable: string) => void
}

export function VariablePicker({ variables, onSelect }: VariablePickerProps) {
  const t = useTranslations("settings.templates")

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button type="button" variant="outline" size="sm">
            <BracesIcon data-icon="inline-start" />
            {t("insertVariable")}
          </Button>
        }
      />
      <PopoverContent className="w-64 p-0" align="start">
        <Command>
          <CommandInput placeholder={t("variableSearch")} />
          <CommandList>
            <CommandEmpty>{t("variableEmpty")}</CommandEmpty>
            <CommandGroup>
              {variables.map((variable) => (
                <CommandItem
                  key={variable}
                  value={variable}
                  onSelect={() => onSelect(variable)}
                >
                  {`{${variable}}`}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
