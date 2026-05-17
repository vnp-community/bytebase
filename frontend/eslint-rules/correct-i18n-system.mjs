/**
 * ESLint Rule: correct-i18n-system
 * Flags: useI18n() in .tsx files (should be useTranslation())
 * Flags: useTranslation() in .vue files (should be useI18n())
 * Ensures each framework uses its correct i18n system.
 */
export const correctI18nSystem = {
  meta: {
    type: "problem",
    docs: {
      description: "Ensure correct i18n hook is used per framework: useTranslation() in .tsx, useI18n() in .vue",
    },
    messages: {
      wrongInReact: "Use `useTranslation()` from react-i18next, not `useI18n()`, in React (.tsx) files.",
      wrongInVue: "Use `useI18n()` from vue-i18n, not `useTranslation()`, in Vue (.vue) files.",
    },
    schema: [],
  },
  create(context) {
    const isVue = context.filename.endsWith(".vue");
    const isReact = context.filename.endsWith(".tsx");

    if (!isVue && !isReact) return {};

    return {
      CallExpression(node) {
        const calleeName = node.callee?.name;
        if (!calleeName) return;

        if (isReact && calleeName === "useI18n") {
          context.report({ node, messageId: "wrongInReact" });
        }

        if (isVue && calleeName === "useTranslation") {
          context.report({ node, messageId: "wrongInVue" });
        }
      },
    };
  },
};
