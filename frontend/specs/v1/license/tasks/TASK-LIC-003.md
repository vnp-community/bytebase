# TASK-LIC-003: Override Zustand Workspace Slice

> **Source**: SOL-LIC-001 §4.3 | **Priority**: P0 | **Effort**: 30 min  
> **Status**: DONE | **Deps**: LIC-001

## Scope
- **EDIT** `src/react/stores/app/workspace.ts`

## What
Override subscription-related methods trong Zustand `WorkspaceSlice` để mirror các thay đổi từ Pinia store (TASK-LIC-002). Slice này phục vụ toàn bộ React-side components qua `useAppStore()`.

## Implementation

### 4.3.1 — Override Plan Methods

```diff
-  currentPlan: () => {
-    return get().subscription?.plan ?? PlanType.FREE;
-  },
+  currentPlan: () => PlanType.ENTERPRISE, // VNP-LIC-001

-  isFreePlan: () => get().currentPlan() === PlanType.FREE,
+  isFreePlan: () => false, // VNP-LIC-001

-  isTrialing: () => Boolean(get().subscription?.trialing),
+  isTrialing: () => false, // VNP-LIC-001
```

### 4.3.2 — Override Expiry Methods

```diff
-  isExpired: () => {
-    const subscription = get().subscription;
-    if (!subscription?.expiresTime || get().isFreePlan()) {
-      return false;
-    }
-    return dayjs(
-      getDateForPbTimestampProtoEs(subscription.expiresTime)
-    ).isBefore(new Date());
-  },
+  isExpired: () => false, // VNP-LIC-001

-  showTrial: () => {
-    if (!isSelfHostLicense()) {
-      return false;
-    }
-    return !get().subscription || get().isFreePlan();
-  },
+  showTrial: () => false, // VNP-LIC-001
```

### 4.3.3 — Override Limit Methods

```diff
-  instanceCountLimit: () => {
-    const subscription = get().subscription;
-    const licenseLimit = subscription?.instances ?? 0;
-    if (licenseLimit > 0) { return licenseLimit; }
-    const planLimit = PLANS.find(...)?.maximumInstanceCount ?? 0;
-    if (planLimit < 0) { return licenseLimit > 0 ? licenseLimit : Number.MAX_VALUE; }
-    return planLimit;
-  },
+  instanceCountLimit: () => Number.MAX_VALUE, // VNP-LIC-001

-  userCountLimit: () => {
-    let limit = PLANS.find(...)?.maximumSeatCount ?? 0;
-    if (limit < 0) { limit = Number.MAX_VALUE; }
-    const seats = get().subscription?.seats ?? 0;
-    if (seats < 0) { return Number.MAX_VALUE; }
-    if (seats === 0) { return limit; }
-    return seats;
-  },
+  userCountLimit: () => Number.MAX_VALUE, // VNP-LIC-001

-  instanceLicenseCount: () => {
-    const count = get().subscription?.activeInstances ?? 0;
-    return count < 0 ? Number.MAX_VALUE : count;
-  },
+  instanceLicenseCount: () => Number.MAX_VALUE, // VNP-LIC-001

-  hasUnifiedInstanceLicense: () => {
-    return get().instanceCountLimit() <= get().instanceLicenseCount();
-  },
+  hasUnifiedInstanceLicense: () => true, // VNP-LIC-001
```

### 4.3.4 — Override Feature Methods

```diff
-  hasFeature: (feature) => {
-    if (get().isExpired()) {
-      return false;
-    }
-    return checkFeature(get().currentPlan(), feature);
-  },
+  hasFeature: (_feature) => true, // VNP-LIC-001

-  hasInstanceFeature: (feature, instance) => {
-    const plan = get().currentPlan();
-    if (plan === PlanType.FREE) { return get().hasFeature(feature); }
-    if (!instance || !instanceLimitFeature.has(feature)) { return get().hasFeature(feature); }
-    return checkInstanceFeature(
-      plan, feature,
-      get().hasUnifiedInstanceLicense() || instance.activation
-    );
-  },
+  hasInstanceFeature: (_feature, _instance) => true, // VNP-LIC-001

-  instanceMissingLicense: (feature, instance) => {
-    if (!instanceLimitFeature.has(feature) || !instance) { return false; }
-    if (get().hasUnifiedInstanceLicense()) { return false; }
-    return get().hasFeature(feature) && !instance.activation;
-  },
+  instanceMissingLicense: (_feature, _instance) => false, // VNP-LIC-001
```

## Notes
- `loadSubscription()`, `refreshSubscription()`, `uploadLicense()` giữ nguyên
- `daysBeforeExpire()`, `expireAt()`, `trialingDays()` giữ nguyên — chúng chỉ dùng bởi banners (suppressed ở TASK-LIC-004)
- `getMinimumRequiredPlan` import từ `plan.ts` → đã override ở TASK-LIC-001

## AC
- [ ] `currentPlan()` = `PlanType.ENTERPRISE`
- [ ] `isFreePlan()` = `false`, `isExpired()` = `false`, `isTrialing()` = `false`
- [ ] `showTrial()` = `false`
- [ ] `instanceCountLimit()` = `Number.MAX_VALUE`, `userCountLimit()` = `Number.MAX_VALUE`
- [ ] `instanceLicenseCount()` = `Number.MAX_VALUE`
- [ ] `hasUnifiedInstanceLicense()` = `true`
- [ ] `hasFeature(anyFeature)` = `true`
- [ ] `hasInstanceFeature(anyFeature, anyInstance)` = `true`
- [ ] `instanceMissingLicense(anyFeature, anyInstance)` = `false`
- [ ] React components (`FeatureBadge`, `FeatureAttention`, etc.) render correctly
- [ ] Update `index.test.ts` expectations nếu có
