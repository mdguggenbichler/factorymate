import { describe, expect, it } from "vitest"

import { formatDateTime, formatTime } from "@/lib/format"

describe("formatDateTime", () => {
  it("uses 24-hour clock without AM/PM", () => {
    const formatted = formatDateTime("2026-08-17T19:31:00Z", "en-GB")
    expect(formatted).not.toMatch(/AM|PM/i)
    expect(formatted).toMatch(/\d{1,2}:\d{2}/)
  })
})

describe("formatTime", () => {
  it("formats time in 24-hour clock", () => {
    const formatted = formatTime("2026-08-17T19:31:00Z", "en-GB")
    expect(formatted).not.toMatch(/AM|PM/i)
    expect(formatted).toMatch(/^\d{1,2}:\d{2}$/)
  })
})
