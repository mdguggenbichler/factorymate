import { FactoryMateLogo } from "@/components/factorymate-logo"

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center">
          <FactoryMateLogo variant="onLight" />
        </div>
        {children}
      </div>
    </div>
  )
}
