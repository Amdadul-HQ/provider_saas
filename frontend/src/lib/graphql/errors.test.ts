import { CombinedError } from "@urql/core"
import { GraphQLError } from "graphql"
import { describe, expect, it } from "vitest"
import { errorCode, errorField } from "./errors"

const gqlError = (extensions: Record<string, unknown>) =>
  new CombinedError({ graphQLErrors: [new GraphQLError("x", { extensions })] })

describe("errorField", () => {
  it("reads extensions.field (register's taken phone/email is 'account')", () => {
    const err = gqlError({ code: "VALIDATION", field: "account" })
    expect(errorCode(err)).toBe("VALIDATION")
    expect(errorField(err)).toBe("account")
  })

  it("is null when no error names a field", () => {
    expect(errorField(gqlError({ code: "VALIDATION" }))).toBeNull()
    expect(errorField(new Error("plain"))).toBeNull()
    expect(errorField(null)).toBeNull()
  })
})
