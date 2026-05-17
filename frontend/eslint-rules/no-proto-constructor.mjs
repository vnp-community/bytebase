/**
 * ESLint Rule: no-proto-constructor
 * Flags: new Database(), new Project(), new Issue() etc.
 * Fix: Use create(DatabaseSchema, {...}) from @bufbuild/protobuf
 */
const protoTypes = new Set([
  "Database", "Project", "Instance", "Issue", "Plan", "Rollout",
  "User", "Group", "Role", "Setting", "Sheet", "Worksheet",
  "Policy", "ReviewConfig", "IdentityProvider", "Release",
  "Revision", "AccessGrant", "Subscription", "Changelog",
]);

export const noProtoConstructor = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow using `new Constructor()` for proto-es types. Use `create(Schema, {...})` instead.",
    },
    messages: {
      useCreate: "Use `create({{ name }}Schema, {...})` instead of `new {{ name }}()`. Import create from '@bufbuild/protobuf'.",
    },
    schema: [],
  },
  create(context) {
    return {
      NewExpression(node) {
        const name = node.callee?.name;
        if (name && protoTypes.has(name)) {
          context.report({
            node,
            messageId: "useCreate",
            data: { name },
          });
        }
      },
    };
  },
};
