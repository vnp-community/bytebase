/**
 * ESLint Rule: max-component-lines
 *
 * Warns when a React component function exceeds 500 lines.
 * Helps prevent new god components after Phase 2 decomposition.
 *
 * A function is considered a React component if:
 *   1. Its name starts with an uppercase letter
 *   2. It's a function declaration or arrow function assigned to a const
 */
export const maxComponentLines = {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Warn when a React component function exceeds the maximum line count.",
    },
    messages: {
      tooLong:
        'Component "{{name}}" has {{lines}} lines (max {{max}}). Extract hooks and sub-components.',
    },
    schema: [
      {
        type: "object",
        properties: {
          max: { type: "integer", minimum: 1, default: 500 },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    const maxLines = context.options[0]?.max ?? 500;

    function getComponentName(node) {
      // function MyComponent() {}
      if (node.id?.name) return node.id.name;
      // const MyComponent = () => {}
      if (
        node.parent?.type === "VariableDeclarator" &&
        node.parent.id?.name
      ) {
        return node.parent.id.name;
      }
      return null;
    }

    function isReactComponent(name) {
      return name && /^[A-Z]/.test(name);
    }

    function checkNode(node) {
      const name = getComponentName(node);
      if (!isReactComponent(name)) return;

      const lines =
        (node.loc?.end.line ?? 0) - (node.loc?.start.line ?? 0) + 1;
      if (lines > maxLines) {
        context.report({
          node,
          messageId: "tooLong",
          data: { name, lines: String(lines), max: String(maxLines) },
        });
      }
    }

    return {
      FunctionDeclaration: checkNode,
      ArrowFunctionExpression: checkNode,
      FunctionExpression: checkNode,
    };
  },
};
