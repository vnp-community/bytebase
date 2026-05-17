import vueI18n from "@intlify/eslint-plugin-vue-i18n";
import vueTsEslintConfig from "@vue/eslint-config-typescript";
import pluginVue from "eslint-plugin-vue";
import { noProtoConstructor } from "./eslint-rules/no-proto-constructor.mjs";
import { noFetchForGrpc } from "./eslint-rules/no-fetch-for-grpc.mjs";
import { requireUpdateMask } from "./eslint-rules/require-update-mask.mjs";
import { reactPageNamedExport } from "./eslint-rules/react-page-named-export.mjs";
import { correctI18nSystem } from "./eslint-rules/correct-i18n-system.mjs";
import { maxComponentLines } from "./eslint-rules/max-component-lines.mjs";

export default [
  ...pluginVue.configs["flat/essential"],
  ...vueTsEslintConfig({
    extends: ["recommended"],
    supportedScriptLangs: {
      ts: true,
      tsx: true,
    },
    rootDir: import.meta.dirname,
  }),
  ...vueI18n.configs["flat/recommended"],
  {
    ignores: ["**/dist/**", "**/node_modules/**", "**/proto-es/**"],
  },
  {
    rules: {
      "no-console": ["error", { allow: ["warn", "error", "debug", "assert"] }],
      "no-debugger": "error",
      "no-empty-pattern": "error",
      "vue/no-ref-as-operand": "error",
      "no-useless-escape": "error",
      "@typescript-eslint/no-empty-interface": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { varsIgnorePattern: "^_", argsIgnorePattern: "^_" },
      ],
      "@intlify/vue-i18n/no-unused-keys": [
        "error",
        {
          src: "./src",
          extensions: [".js", ".vue", ".ts", ".tsx"],
          ignores: [
            // Used in React .tsx — vue-i18n linter can't detect these
            "project.batch.selected",
            "project.batch.archive.title",
            "project.batch.archive.success",
            "project.batch.delete.title",
            "project.batch.delete.success",
            "sql-review.select-review-rules",
            "sql-review.select-all",
            "sql-review.attach-resource.label-environment",
            "sql-review.attach-resource.label-project",
            "sql-review.attach-resource.override-warning",
            "sql-review.create.basic-info.display-name-placeholder",
            "sql-review.create.basic-info.choose-template",
          ],
          enableFix: true,
        },
      ],
      "@intlify/vue-i18n/no-missing-keys": "error",
      "@intlify/vue-i18n/no-raw-text": "off",
      "@typescript-eslint/no-explicit-any": "error",
      "vue/no-mutating-props": "error",
      "vue/no-unused-components": "error",
      "vue/no-useless-template-attributes": "error",
      "vue/no-undef-components": [
        "error",
        {
          ignorePatterns: [
            /^heroicons(-solid|-outline)?:/,
            /^carbon:/,
            /^tabler:/,
            /^octicon:/,
            /^router-view$/,
            /^router-link$/,
            /^i18n-t$/,
            /^highlight-code-block$/,
          ],
        },
      ],
      "vue/multi-word-component-names": "off",
    },
    settings: {
      "vue-i18n": {
        localeDir: "./src/locales/*.json",
        messageSyntaxVersion: "^9.0.0",
      },
    },
  },
  // React .tsx files use their own locale files (src/react/locales/),
  // so disable vue-i18n missing-keys checks for them.
  {
    files: ["src/react/**/*.tsx"],
    rules: {
      "@intlify/vue-i18n/no-missing-keys": "off",
    },
  },
  // Bytebase AI guardrail rules — prevent top AI coding mistakes
  {
    plugins: {
      "bytebase-ai": {
        rules: {
          "no-proto-constructor": noProtoConstructor,
          "no-fetch-for-grpc": noFetchForGrpc,
          "require-update-mask": requireUpdateMask,
          "correct-i18n-system": correctI18nSystem,
        },
      },
    },
    rules: {
      "bytebase-ai/no-proto-constructor": "error",
      "bytebase-ai/no-fetch-for-grpc": "error",
      "bytebase-ai/require-update-mask": "warn",
      "bytebase-ai/correct-i18n-system": "error",
    },
  },
  // Named export enforcement for React page files only
  {
    files: ["src/react/pages/**/*.tsx"],
    ignores: ["**/*.test.*"],
    plugins: {
      "bytebase-pages": {
        rules: {
          "react-page-named-export": reactPageNamedExport,
        },
      },
    },
    rules: {
      "bytebase-pages/react-page-named-export": "error",
    },
  },
  // Max component size enforcement — prevents future god components
  {
    files: ["src/react/**/*.tsx"],
    ignores: ["**/*.test.*", "src/react/templates/**"],
    plugins: {
      "bytebase-size": {
        rules: {
          "max-component-lines": maxComponentLines,
        },
      },
    },
    rules: {
      "bytebase-size/max-component-lines": ["warn", { max: 500 }],
    },
  },
  // Exclude template files from lint (they contain placeholder code)
  {
    ignores: ["src/react/templates/**"],
  },
];

