/**
 * ESLint Rule: require-update-mask
 * Flags: client.updateXxx({...}) without updateMask property
 * All ConnectRPC update calls MUST include updateMask to specify changed fields.
 */
export const requireUpdateMask = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Require updateMask field in ConnectRPC update method calls.",
    },
    messages: {
      missingUpdateMask: "`{{ method }}()` requires an `updateMask` field. List only the changed fields to avoid overwriting unintended properties.",
    },
    schema: [],
  },
  create(context) {
    return {
      CallExpression(node) {
        const callee = node.callee;
        // Match: something.updateXxx({...})
        if (callee.type !== "MemberExpression") return;

        const methodName = callee.property?.name ?? "";
        if (!/^update[A-Z]/.test(methodName)) return;

        // Check first argument is an object
        const firstArg = node.arguments[0];
        if (!firstArg || firstArg.type !== "ObjectExpression") return;

        // Check if updateMask property exists
        const hasUpdateMask = firstArg.properties.some((prop) => {
          if (prop.type === "SpreadElement") return false;
          const key = prop.key;
          return (
            (key?.type === "Identifier" && key.name === "updateMask") ||
            (key?.type === "Literal" && key.value === "updateMask")
          );
        });

        if (!hasUpdateMask) {
          context.report({
            node,
            messageId: "missingUpdateMask",
            data: { method: methodName },
          });
        }
      },
    };
  },
};
