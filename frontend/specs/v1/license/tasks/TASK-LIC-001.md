# TASK-LIC-001: Override Core Feature Check (`plan.ts`)

> **Source**: SOL-LIC-001 §4.1 | **Priority**: P0 | **Effort**: 15 min  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/types/plan.ts`

## What
Override 3 core functions trong `plan.ts` để mọi feature check luôn return `true`, bỏ qua kiểm tra license plan. Đây là điểm thay đổi quan trọng nhất — tất cả consumers (Pinia store, Zustand store, components) đều gọi qua các functions này.

## Implementation

```diff
 // Helper function to check if a feature is available for a plan
-export const hasFeature = (plan: PlanType, feature: PlanFeature): boolean => {
-  return planHasFeature(plan, feature);
-};
+export const hasFeature = (_plan: PlanType, _feature: PlanFeature): boolean => {
+  // VNP-LIC-001: Bypass license check — all features enabled
+  return true;
+};

 // Helper function to get minimum required plan for a feature
-export const getMinimumRequiredPlan = (feature: PlanFeature): PlanType => {
-  const planOrder = [PlanType.FREE, PlanType.TEAM, PlanType.ENTERPRISE];
-  for (const plan of planOrder) {
-    if (planHasFeature(plan, feature)) {
-      return plan;
-    }
-  }
-  return PlanType.ENTERPRISE;
-};
+export const getMinimumRequiredPlan = (_feature: PlanFeature): PlanType => {
+  // VNP-LIC-001: All features available at FREE level
+  return PlanType.FREE;
+};

 // Helper function to check instance features
-export const hasInstanceFeature = (
-  plan: PlanType,
-  feature: PlanFeature,
-  instanceActivated = true
-): boolean => {
-  if (!hasFeature(plan, feature)) {
-    return false;
-  }
-  if (plan === PlanType.FREE) {
-    return true;
-  }
-  if (instanceLimitFeature.has(feature)) {
-    return instanceActivated;
-  }
-  return true;
-};
+export const hasInstanceFeature = (
+  _plan: PlanType,
+  _feature: PlanFeature,
+  _instanceActivated = true
+): boolean => {
+  // VNP-LIC-001: All instance features enabled regardless of activation
+  return true;
+};
```

## Notes
- Giữ nguyên `planHasFeature()` (private), `PLANS`, `planFeatureMatrix`, `instanceLimitFeature` — không xóa, để dễ rollback.
- Tag `VNP-LIC-001` cho mọi comment — dễ grep khi rollback.
- `plan.yaml` **không cần sửa** — data vẫn giữ nguyên.

## AC
- [ ] `hasFeature()` luôn return `true` cho mọi `PlanType` + `PlanFeature` combination
- [ ] `getMinimumRequiredPlan()` luôn return `PlanType.FREE`
- [ ] `hasInstanceFeature()` luôn return `true` bất kể `instanceActivated`
- [ ] Existing unit tests in `subscription.test.ts` cần update expectations
- [ ] TypeScript build thành công (no type errors)
