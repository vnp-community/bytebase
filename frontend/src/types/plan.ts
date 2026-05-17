import { create } from "@bufbuild/protobuf";
import {
  PlanFeature,
  type PlanLimitConfig,
  PlanLimitConfigSchema,
  PlanType,
} from "@/types/proto-es/v1/subscription_service_pb";
import planData from "./plan.yaml";

// Type for plan data loaded from YAML
interface PlanYamlData {
  type: keyof typeof PlanType;
  maximumInstanceCount: number;
  maximumSeatCount: number;
  features: (keyof typeof PlanFeature)[];
}

interface PlanDataYaml {
  plans: PlanYamlData[];
  instanceFeatures: string[];
}

const typedPlanData = planData as unknown as PlanDataYaml;

// Convert YAML data to proper types
export const PLANS: PlanLimitConfig[] = typedPlanData.plans.map((plan) =>
  create(PlanLimitConfigSchema, {
    type: PlanType[plan.type],
    features: plan.features.map((f) => PlanFeature[f]),
    maximumInstanceCount: plan.maximumInstanceCount,
    maximumSeatCount: plan.maximumSeatCount,
  })
);

// Create a plan feature matrix from the YAML data
const planFeatureMatrix = new Map<PlanType, Set<PlanFeature>>();
// Instance-limited features that require activated instances
export const instanceLimitFeature = new Set<PlanFeature>();

// Initialize the feature matrix and instance features from plan data
PLANS.forEach((plan) => {
  planFeatureMatrix.set(plan.type, new Set(plan.features));
});
typedPlanData.instanceFeatures.forEach((feature) => {
  instanceLimitFeature.add(PlanFeature[feature as keyof typeof PlanFeature]);
});

// Helper function to check if a plan has a feature
const planHasFeature = (plan: PlanType, feature: PlanFeature): boolean => {
  const planFeatures = planFeatureMatrix.get(plan);
  return planFeatures?.has(feature) ?? false;
};

// Helper function to get minimum required plan for a feature
export const getMinimumRequiredPlan = (_feature: PlanFeature): PlanType => {
  // VNP-LIC-001: All features available at FREE level
  return PlanType.FREE;
};

// Helper function to check if a feature is available for a plan
export const hasFeature = (_plan: PlanType, _feature: PlanFeature): boolean => {
  // VNP-LIC-001: Bypass license check — all features enabled
  return true;
};

// Helper function to check instance features
export const hasInstanceFeature = (
  _plan: PlanType,
  _feature: PlanFeature,
  _instanceActivated = true
): boolean => {
  // VNP-LIC-001: All instance features enabled regardless of activation
  return true;
};
