import type { CodegenConfig } from "@graphql-codegen/cli"

// Schema is the backend contract (ADR-003). Never hand-edit src/gql/.
const config: CodegenConfig = {
  schema: ["../backend/graph/schema/*.graphql", "../backend/internal/modules/*/*.graphql"],
  documents: ["src/modules/**/graphql/*.graphql"],
  ignoreNoDocuments: false,
  generates: {
    "./src/gql/": {
      preset: "client",
      presetConfig: { fragmentMasking: false },
      config: {
        enumsAsConst: true,
        useTypeImports: true,
        avoidOptionals: false,
        scalars: {
          Time: "string",
          UUID: "string",
          Money: "number",
          JSON: "unknown",
        },
      },
    },
  },
}

export default config
