/**
 * AI REFERENCE — User Domain
 * Full type: src/types/proto-es/v1/user_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed User fields for AI reference */
export interface UserRef {
  /** Resource name: "users/{email}" */
  name: string;
  /** Email address (unique identifier) */
  email: string;
  /** Display name */
  title: string;
  /** Workspace role */
  role: "OWNER" | "DBA" | "DEVELOPER";
  /** Whether MFA is enabled */
  mfaEnabled: boolean;
  /** Phone number */
  phone: string;
  /** User type */
  userType: "USER" | "SYSTEM_BOT" | "SERVICE_ACCOUNT";
}

export const USER_CLIENT = "userServiceClientConnect" as const;
export const USER_UPDATE_MASK_FIELDS = ["title", "email", "phone", "password", "mfaEnabled"] as const;

/**
 * Service methods:
 * - getUser({ name }) → User
 * - listUsers({ pageSize?, filter?, showDeleted? }) → { users, nextPageToken }
 * - createUser({ user }) → User
 * - updateUser({ user, updateMask }) → User
 * - deleteUser({ name }) → Empty
 */
