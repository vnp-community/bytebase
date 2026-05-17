/**
 * Centralized query key factory for all 24+ domains.
 *
 * Naming convention:
 *   queryKeys.<domain>.all   → invalidate everything for the domain
 *   queryKeys.<domain>.list  → list queries (accepts parent/filter)
 *   queryKeys.<domain>.detail → single resource by name
 *
 * Usage:
 *   useQuery({ queryKey: queryKeys.database.detail("instances/i1/databases/d1"), ... })
 */
export const queryKeys = {
  // ── Core resources ────────────────────────────────
  database: {
    all: ["databases"] as const,
    list: (parent: string) => ["databases", parent] as const,
    detail: (name: string) => ["database", name] as const,
  },
  project: {
    all: ["projects"] as const,
    list: () => ["projects"] as const,
    detail: (name: string) => ["project", name] as const,
    iamPolicy: (name: string) => ["project", name, "iamPolicy"] as const,
  },
  instance: {
    all: ["instances"] as const,
    list: (parent?: string) => ["instances", parent ?? "all"] as const,
    detail: (name: string) => ["instance", name] as const,
  },
  user: {
    all: ["users"] as const,
    list: () => ["users"] as const,
    detail: (name: string) => ["user", name] as const,
  },
  environment: {
    all: ["environments"] as const,
    list: () => ["environments"] as const,
    detail: (name: string) => ["environment", name] as const,
  },

  // ── Issue / Plan / Rollout ────────────────────────
  issue: {
    all: ["issues"] as const,
    list: (project: string, filter?: string) =>
      ["issues", project, filter] as const,
    detail: (name: string) => ["issue", name] as const,
  },
  plan: {
    all: ["plans"] as const,
    list: (project: string) => ["plans", project] as const,
    detail: (name: string) => ["plan", name] as const,
  },
  rollout: {
    all: ["rollouts"] as const,
    detail: (name: string) => ["rollout", name] as const,
    stages: (rolloutName: string) =>
      ["rollout", rolloutName, "stages"] as const,
  },

  // ── IAM / Auth ────────────────────────────────────
  role: {
    all: ["roles"] as const,
    list: () => ["roles"] as const,
  },
  group: {
    all: ["groups"] as const,
    list: () => ["groups"] as const,
    detail: (name: string) => ["group", name] as const,
  },
  idp: {
    all: ["idps"] as const,
    list: () => ["idps"] as const,
    detail: (name: string) => ["idp", name] as const,
  },
  accessGrant: {
    all: ["accessGrants"] as const,
    list: (project: string) => ["accessGrants", project] as const,
  },
  serviceAccount: {
    all: ["serviceAccounts"] as const,
    list: () => ["serviceAccounts"] as const,
  },
  workloadIdentity: {
    all: ["workloadIdentities"] as const,
    list: () => ["workloadIdentities"] as const,
  },

  // ── Settings / Config ─────────────────────────────
  setting: {
    all: ["settings"] as const,
    byName: (settingName: string) => ["setting", settingName] as const,
  },
  subscription: {
    all: ["subscription"] as const,
    current: () => ["subscription", "current"] as const,
  },
  policy: {
    all: ["policies"] as const,
    list: (parent: string) => ["policies", parent] as const,
  },
  reviewConfig: {
    all: ["reviewConfigs"] as const,
    list: () => ["reviewConfigs"] as const,
    detail: (name: string) => ["reviewConfig", name] as const,
  },

  // ── SQL Editor / Worksheets ───────────────────────
  worksheet: {
    all: ["worksheets"] as const,
    list: (parent?: string) => ["worksheets", parent ?? "all"] as const,
    detail: (name: string) => ["worksheet", name] as const,
  },
  schema: {
    all: ["schemas"] as const,
    detail: (database: string) => ["schema", database] as const,
    metadata: (database: string) => ["schema", database, "metadata"] as const,
  },

  // ── Audit / Changelog / Release ───────────────────
  auditLog: {
    all: ["auditLogs"] as const,
    list: (filter?: string) => ["auditLogs", filter] as const,
  },
  changelog: {
    all: ["changelogs"] as const,
    list: (database: string) => ["changelogs", database] as const,
    detail: (name: string) => ["changelog", name] as const,
  },
  release: {
    all: ["releases"] as const,
    list: (project: string) => ["releases", project] as const,
    detail: (name: string) => ["release", name] as const,
  },
  databaseGroup: {
    all: ["databaseGroups"] as const,
    list: (project: string) => ["databaseGroups", project] as const,
    detail: (name: string) => ["databaseGroup", name] as const,
  },
} as const;
