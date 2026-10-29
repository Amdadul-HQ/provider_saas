"use client"

import { useState, type ReactNode } from "react"
import { Provider } from "urql"
import { makeClient } from "./client"

export function GraphQLProvider({ children }: { children: ReactNode }) {
  const [client] = useState(makeClient)
  return <Provider value={client}>{children}</Provider>
}
