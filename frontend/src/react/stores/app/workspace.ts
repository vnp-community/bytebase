import { create as createProto } from "@bufbuild/protobuf";
import dayjs from "dayjs";
import semver from "semver";
import {
  actuatorServiceClientConnect,
  settingServiceClientConnect,
  subscriptionServiceClientConnect,
  workspaceServiceClientConnect,
} from "@/connect";
import {
  settingNamePrefix,
  workspaceNamePrefix,
} from "@/react/lib/resourceName";
import { defaultAppProfile } from "@/types/appProfile";
import {
  hasFeature as checkFeature,
  hasInstanceFeature as checkInstanceFeature,
  getMinimumRequiredPlan,
  instanceLimitFeature,
  PLANS,
} from "@/types/plan";
import {
  DatabaseChangeMode,
  type EnvironmentSetting_Environment,
  EnvironmentSetting_EnvironmentSchema,
  GetSettingRequestSchema,
  Setting_SettingName,
  WorkspaceProfileSettingSchema,
} from "@/types/proto-es/v1/setting_service_pb";
import {
  GetSubscriptionRequestSchema,
  PlanType,
  UploadLicenseRequestSchema,
} from "@/types/proto-es/v1/subscription_service_pb";
import {
  getDateForPbTimestampProtoEs,
  getTimeForPbTimestampProtoEs,
} from "@/types/timestamp";
import type { Environment } from "@/types/v1/environment";
import { formatAbsoluteDateTime } from "@/utils/datetime";
import type { AppSliceCreator, WorkspaceSlice } from "./types";

const workspaceProfileSettingName = `${settingNamePrefix}${
  Setting_SettingName[Setting_SettingName.WORKSPACE_PROFILE]
}`;
const environmentSettingName = `${settingNamePrefix}${
  Setting_SettingName[Setting_SettingName.ENVIRONMENT]
}`;
const externalUrlPlaceholder =
  "https://docs.bytebase.com/get-started/self-host/external-url";
const trialingDays = 14;

function appFeaturesFromDatabaseChangeMode(mode: DatabaseChangeMode) {
  const appFeatures = defaultAppProfile().features;
  appFeatures["bb.feature.database-change-mode"] =
    mode === DatabaseChangeMode.EDITOR
      ? DatabaseChangeMode.EDITOR
      : DatabaseChangeMode.PIPELINE;
  if (
    appFeatures["bb.feature.database-change-mode"] === DatabaseChangeMode.EDITOR
  ) {
    appFeatures["bb.feature.hide-quick-start"] = true;
    appFeatures["bb.feature.hide-trial"] = true;
  }
  return appFeatures;
}

function environmentNameFromId(id: string) {
  return `environments/${id}`;
}

function convertEnvironmentList(
  environments: EnvironmentSetting_Environment[]
): Environment[] {
  return environments.map<Environment>((env, i) => ({
    ...createProto(EnvironmentSetting_EnvironmentSchema, {
      name: environmentNameFromId(env.id),
      id: env.id,
      title: env.title,
      color: env.color,
      tags: env.tags,
    }),
    order: i,
  }));
}

function isSelfHostLicense() {
  return import.meta.env.MODE.toLowerCase() !== "release-aws";
}

export const createWorkspaceSlice: AppSliceCreator<WorkspaceSlice> = (
  set,
  get
) => ({
  serverInfoTs: 0,
  environmentList: [],
  appFeatures: defaultAppProfile().features,

  loadServerInfo: async () => {
    const existing = get().serverInfo;
    if (existing) return existing;
    const pending = get().serverInfoRequest;
    if (pending) return pending;
    const request = actuatorServiceClientConnect
      .getActuatorInfo({ name: get().currentUser?.workspace ?? "" })
      .then((info) => {
        set({
          serverInfo: info,
          serverInfoRequest: undefined,
          serverInfoTs: Date.now(),
        });
        return info;
      })
      .catch(() => {
        set({ serverInfoRequest: undefined });
        return undefined;
      });
    set({ serverInfoRequest: request });
    return request;
  },

  refreshServerInfo: async () => {
    const info = await actuatorServiceClientConnect.getActuatorInfo({
      name: get().currentUser?.workspace ?? get().serverInfo?.workspace ?? "",
    });
    set({ serverInfo: info, serverInfoTs: Date.now() });
    return info;
  },

  loadWorkspace: async () => {
    const existing = get().workspace;
    if (existing) return existing;
    const pending = get().workspaceRequest;
    if (pending) return pending;
    await Promise.all([get().loadCurrentUser(), get().loadServerInfo()]);
    const name =
      get().serverInfo?.workspace ||
      get().currentUser?.workspace ||
      `${workspaceNamePrefix}-`;
    const request = workspaceServiceClientConnect
      .getWorkspace({ name })
      .then((workspace) => {
        set({ workspace, workspaceRequest: undefined });
        return workspace;
      })
      .catch(() => {
        set({ workspaceRequest: undefined });
        return undefined;
      });
    set({ workspaceRequest: request });
    return request;
  },

  loadWorkspaceProfile: async () => {
    const existing = get().workspaceProfile;
    if (existing) return existing;
    const pending = get().workspaceProfileRequest;
    if (pending) return pending;
    const request = settingServiceClientConnect
      .getSetting(
        createProto(GetSettingRequestSchema, {
          name: workspaceProfileSettingName,
        })
      )
      .then((setting) => {
        const settingValue = setting.value?.value;
        const profile =
          settingValue?.case === "workspaceProfile"
            ? settingValue.value
            : createProto(WorkspaceProfileSettingSchema, {});
        set({
          workspaceProfile: profile,
          workspaceProfileRequest: undefined,
          appFeatures: appFeaturesFromDatabaseChangeMode(
            profile.databaseChangeMode
          ),
        });
        return profile;
      })
      .catch(() => {
        set({ workspaceProfileRequest: undefined });
        return undefined;
      });
    set({ workspaceProfileRequest: request });
    return request;
  },

  loadEnvironmentList: async (force = false) => {
    const existing = get().environmentList;
    if (!force && existing.length > 0) return existing;
    const pending = get().environmentRequest;
    if (pending) return pending;
    const request = settingServiceClientConnect
      .getSetting(
        createProto(GetSettingRequestSchema, {
          name: environmentSettingName,
        })
      )
      .then((setting) => {
        const settingValue = setting.value?.value;
        const environments =
          settingValue?.case === "environment"
            ? convertEnvironmentList(settingValue.value.environments)
            : [];
        set({ environmentList: environments, environmentRequest: undefined });
        return environments;
      })
      .catch(() => {
        set({ environmentRequest: undefined });
        return [];
      });
    set({ environmentRequest: request });
    return request;
  },

  refreshEnvironmentList: async () => get().loadEnvironmentList(true),

  loadSubscription: async () => {
    const existing = get().subscription;
    if (existing) return existing;
    const pending = get().subscriptionRequest;
    if (pending) return pending;
    const request = subscriptionServiceClientConnect
      .getSubscription(createProto(GetSubscriptionRequestSchema, {}))
      .then((subscription) => {
        set({ subscription, subscriptionRequest: undefined });
        return subscription;
      })
      .catch(() => {
        set({ subscriptionRequest: undefined });
        return undefined;
      });
    set({ subscriptionRequest: request });
    return request;
  },

  refreshSubscription: async () => {
    const request = subscriptionServiceClientConnect
      .getSubscription(createProto(GetSubscriptionRequestSchema, {}))
      .then((subscription) => {
        set({ subscription, subscriptionRequest: undefined });
        return subscription;
      })
      .catch(() => {
        set({ subscriptionRequest: undefined });
        return undefined;
      });
    set({ subscriptionRequest: request });
    return request;
  },

  uploadLicense: async (license) => {
    const subscription = await subscriptionServiceClientConnect.uploadLicense(
      createProto(UploadLicenseRequestSchema, { license })
    );
    set({ subscription });
    return subscription;
  },

  currentPlan: () => PlanType.ENTERPRISE, // VNP-LIC-001

  isFreePlan: () => false, // VNP-LIC-001

  isTrialing: () => false, // VNP-LIC-001

  isExpired: () => false, // VNP-LIC-001

  daysBeforeExpire: () => {
    const subscription = get().subscription;
    if (!subscription?.expiresTime || get().isFreePlan()) {
      return -1;
    }
    return Math.max(
      dayjs(getDateForPbTimestampProtoEs(subscription.expiresTime)).diff(
        new Date(),
        "day"
      ),
      0
    );
  },

  trialingDays: () => trialingDays,

  showTrial: () => false, // VNP-LIC-001

  expireAt: () => {
    const subscription = get().subscription;
    if (!subscription?.expiresTime || get().isFreePlan()) {
      return "";
    }
    return formatAbsoluteDateTime(
      getTimeForPbTimestampProtoEs(subscription.expiresTime)
    );
  },

  instanceCountLimit: () => Number.MAX_VALUE, // VNP-LIC-001

  userCountLimit: () => Number.MAX_VALUE, // VNP-LIC-001

  instanceLicenseCount: () => Number.MAX_VALUE, // VNP-LIC-001

  hasUnifiedInstanceLicense: () => true, // VNP-LIC-001

  hasFeature: (_feature) => true, // VNP-LIC-001

  hasInstanceFeature: (_feature, _instance) => true, // VNP-LIC-001

  instanceMissingLicense: (_feature, _instance) => false, // VNP-LIC-001

  getMinimumRequiredPlan,

  isSaaSMode: () => get().serverInfo?.saas ?? false,

  workspaceResourceName: () => get().serverInfo?.workspace ?? "",

  externalUrl: () => get().serverInfo?.externalUrl ?? "",

  needConfigureExternalUrl: () => {
    const serverInfo = get().serverInfo;
    if (!serverInfo) return false;
    const url = serverInfo.externalUrl ?? "";
    return url === "" || url === externalUrlPlaceholder;
  },

  version: () => get().serverInfo?.version ?? "",

  changelogURL: () => {
    const version = semver.valid(get().serverInfo?.version);
    if (!version) return "";
    return `https://docs.bytebase.com/changelog/bytebase-${version
      .split(".")
      .join("-")}/`;
  },

  activatedInstanceCount: () => get().serverInfo?.activatedInstanceCount ?? 0,

  totalInstanceCount: () => get().serverInfo?.totalInstanceCount ?? 0,

  userCountInIam: () => get().serverInfo?.userCountInIam ?? 0,
});
