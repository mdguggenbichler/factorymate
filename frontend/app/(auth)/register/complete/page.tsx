import { Suspense } from "react"
import { RegisterCompleteForm } from "@/components/register-complete-form"

export default function RegisterCompletePage() {
  return (
    <Suspense fallback={null}>
      <RegisterCompleteForm />
    </Suspense>
  )
}
