/**
 * ESLint Rule: no-fetch-for-grpc
 * Flags: fetch('/v1/...') — should use ConnectRPC service client
 * See: .ai-context/CONNECTRPC_GUIDE.md for correct patterns
 */
export const noFetchForGrpc = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow using fetch() for gRPC endpoints. Use ConnectRPC service clients.",
    },
    messages: {
      useConnectRPC: "Use ConnectRPC service client instead of fetch() for /v1/ endpoints. See .ai-context/CONNECTRPC_GUIDE.md",
    },
    schema: [],
  },
  create(context) {
    return {
      CallExpression(node) {
        // Match: fetch("/v1/...") or fetch('/v1/...')
        const callee = node.callee;
        const isFetch =
          callee.name === "fetch" ||
          (callee.type === "MemberExpression" &&
            callee.property?.name === "fetch");

        if (!isFetch) return;

        const firstArg = node.arguments[0];
        if (!firstArg) return;

        // String literal
        if (
          firstArg.type === "Literal" &&
          typeof firstArg.value === "string" &&
          firstArg.value.startsWith("/v1/")
        ) {
          context.report({ node, messageId: "useConnectRPC" });
          return;
        }

        // Template literal: fetch(`/v1/...`)
        if (
          firstArg.type === "TemplateLiteral" &&
          firstArg.quasis[0]?.value?.raw?.startsWith("/v1/")
        ) {
          context.report({ node, messageId: "useConnectRPC" });
        }
      },
    };
  },
};
