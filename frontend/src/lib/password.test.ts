import { describe, expect, it } from "vitest"
import { passwordChecks, passwordScore } from "./password"

describe("passwordScore", () => {
  it.each([
    ["", 0],
    ["abc", 1],
    ["abcdefgh", 1],
    ["12345678", 1],
    ["abcd1234", 2],
    ["abcd1234!", 3],
    ["abcdefgh1234", 3],
    ["আমার১২৩৪৫", 2],
    ["আমার১২৩৪৫!", 3],
  ] as const)("%s → %i", (p, want) => expect(passwordScore(p)).toBe(want))

  it("reports each rule", () => {
    expect(passwordChecks("ab1")).toEqual({ length: false, letterDigit: true, extra: false })
  })
})
