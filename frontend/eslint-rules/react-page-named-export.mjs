/**
 * ESLint Rule: react-page-named-export
 * Flags: Page files in src/react/pages/ that don't export a named function matching the filename.
 * This ensures mount.ts name lookup resolves correctly.
 */
import path from "path";

export const reactPageNamedExport = {
  meta: {
    type: "problem",
    docs: {
      description: "React page files must have a named export matching the filename (not export default).",
    },
    messages: {
      missingNamedExport: 'Page file must export named function "{{ expected }}". Do not use "export default". See .ai-context/NEW_PAGE_PLAYBOOK.md',
    },
    schema: [],
  },
  create(context) {
    // Only apply to files in src/react/pages/
    if (!context.filename.includes("/pages/")) return {};
    if (!context.filename.endsWith(".tsx")) return {};

    // Skip test files
    if (context.filename.includes(".test.")) return {};

    const expected = path.basename(context.filename, ".tsx");

    return {
      "Program:exit"(node) {
        const hasNamedExport = node.body.some((statement) => {
          if (statement.type !== "ExportNamedDeclaration") return false;

          // export function PageName() { ... }
          if (statement.declaration?.id?.name === expected) return true;

          // export { PageName }
          if (
            statement.specifiers?.some(
              (sp) => sp.exported?.name === expected
            )
          )
            return true;

          // export const PageName = ...
          if (
            statement.declaration?.declarations?.some(
              (d) => d.id?.name === expected
            )
          )
            return true;

          return false;
        });

        if (!hasNamedExport) {
          context.report({
            node,
            messageId: "missingNamedExport",
            data: { expected },
          });
        }
      },
    };
  },
};
