import { fileURLToPath, URL } from "node:url";
// @ts-ignore -- esbuild is a transitive dependency via vite
import { transform as esbuildTransform } from "esbuild";
import importMetaUrlPlugin from "@codingame/esbuild-import-meta-url-plugin";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import yaml from "@rollup/plugin-yaml";
import tailwindcss from "@tailwindcss/vite";
import legacy from "@vitejs/plugin-legacy";
import vue from "@vitejs/plugin-vue";
import vueJsx from "@vitejs/plugin-vue-jsx";
import { CodeInspectorPlugin } from "code-inspector-plugin";
import { resolve } from "path";
import Components from "unplugin-vue-components/vite";
import { defineConfig } from "vite";
import { exportCspHashes } from "./vite-plugin-export-csp-hashes";

const SERVER_PORT = parseInt(process.env.PORT ?? "3000", 10) ?? 3000;
const LOCAL_ENDPOINT = "http://localhost:8080";

const extractHostPort = (url: string) => {
  const parsed = new URL(url);
  return parsed.host;
};

export default defineConfig(({ mode }) => ({
  // TASK-W-027: Strip console.debug in production builds
  esbuild: {
    pure: mode === "production" ? ["console.debug"] : [],
  },
  plugins: [
    legacy({
      targets: ["Chrome >= 84, Firefox >= 79, Safari >= 14.1, Edge >= 84"],
      additionalLegacyPolyfills: ["regenerator-runtime/runtime"],
    }),
    {
      name: "react-tsx-transform",
      enforce: "pre",
      async transform(code, id) {
        if (!/\/src\/react\/.+\.tsx$/.test(id)) return undefined;
        const result = await esbuildTransform(code, {
          loader: "tsx",
          jsx: "automatic",
          jsxImportSource: "react",
          tsconfigRaw: {
            compilerOptions: {
              strict: true,
              target: "ES2022",
              useDefineForClassFields: true,
            }
          },
          sourcemap: true,
          sourcefile: id,
        });
        return { code: result.code, map: result.map || null };
      },
    },
    vue(),
    vueJsx({
      include: /\.tsx$/,
      exclude: /src\/react\//,
    }),
    // https://github.com/intlify/vite-plugin-vue-i18n
    VueI18nPlugin({
      include: [resolve(__dirname, "src/locales/**")],
      strictMessage: false,
    }),
    tailwindcss(),
    Components({
      allowOverrides: true,
    }),
    yaml(),
    ...(process.env.VITEST
      ? []
      : [
          CodeInspectorPlugin({
            bundler: "vite",
            exclude: [/src\/react\//],
          }),
        ]),
    // Export CSP hashes from @vitejs/plugin-legacy for backend to use
    exportCspHashes(),
  ],
  build: {
    chunkSizeWarningLimit: 500,
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
        "explain-visualizer": resolve(__dirname, "explain-visualizer.html"),
      },
      output: {
        manualChunks: (id) => {
          if (id.includes("node_modules")) {
            const pkgMatch = id.match(/node_modules\/(.+?)\//);
            if (!pkgMatch) return undefined;
            const pkg = pkgMatch[1];
            
            if (pkg.startsWith("@codingame/monaco") || pkg === "monaco-editor") return "monaco-editor";
            if (["sql-formatter", "antlr4"].includes(pkg)) return "sql-tools";
            if (pkg === "naive-ui") return "ui-framework";
            if (["lodash-es", "dayjs"].includes(pkg)) return "utils";
            if (["react", "react-dom", "react-i18next", "i18next"].includes(pkg)) return "react-core";
          }
        },
      },
    },
  },
  server: {
    port: SERVER_PORT,
    host: "0.0.0.0",
    proxy: {
      "/v1:adminExecute": {
        target: `ws://${extractHostPort(LOCAL_ENDPOINT)}/`,
        changeOrigin: true,
        ws: true,
      },
      "/lsp": {
        target: `ws://${extractHostPort(LOCAL_ENDPOINT)}/`,
        changeOrigin: true,
        ws: true,
      },
      "/api": {
        target: `${LOCAL_ENDPOINT}/api`,
        changeOrigin: true,
        rewrite: (path: string) => path.replace(/^\/api/, ""),
      },
      "/hook": {
        target: LOCAL_ENDPOINT,
        changeOrigin: true,
      },
      "/v1": {
        target: `${LOCAL_ENDPOINT}/v1`,
        changeOrigin: true,
        rewrite: (path: string) => path.replace(/^\/v1/, ""),
      },
    },
    hmr: {
      port: SERVER_PORT,
    },
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  optimizeDeps: {
    include: ["vscode-textmate", "vscode-oniguruma"],
    esbuildOptions: {
      plugins: [importMetaUrlPlugin],
    },
  },
  envPrefix: ["BB_", "GIT_COMMIT"],
  define: {
    _global: {},
  },
  worker: {
    format: "es",
  },
}));
