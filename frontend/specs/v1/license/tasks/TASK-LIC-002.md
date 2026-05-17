# TASK-LIC-002: Override Pinia Subscription Store

> **Source**: SOL-LIC-001 §4.2 | **Priority**: P0 | **Effort**: 30 min  
> **Status**: DONE | **Deps**: LIC-001

## Scope
- **EDIT** `src/store/modules/v1/subscription.ts`

## What
Override computed getters và action methods trong Pinia `useSubscriptionV1Store` để report Enterprise plan, unlimited limits, và bypass tất cả feature/license checks. Store này phục vụ toàn bộ Vue-side components.

## Implementation

### 4.2.1 — Override Plan Getters

```diff
   const currentPlan = computed(() => {
-    if (!subscription.value) {
-      return PlanType.FREE;
-    }
-    return subscription.value.plan;
+    // VNP-LIC-001: Always report Enterprise plan
+    return PlanType.ENTERPRISE;
   });

-  const isFreePlan = computed(() => currentPlan.value === PlanType.FREE);
+  const isFreePlan = computed(() => false); // VNP-LIC-001
```

### 4.2.2 — Override Limit Getters

```diff
   const instanceCountLimit = computed(() => {
-    let limit = subscription.value?.instances ?? 0;
-    if (limit > 0) { return limit; }
-    limit = PLANS.find((plan) => plan.type === currentPlan.value)
-      ?.maximumInstanceCount ?? 0;
-    if (limit < 0) {
-      const instanceLimitInLicense = subscription.value?.instances ?? 0;
-      if (instanceLimitInLicense > 0) { return instanceLimitInLicense; }
-      return Number.MAX_VALUE;
-    }
-    return limit;
+    // VNP-LIC-001: Unlimited instances
+    return Number.MAX_VALUE;
   });

   const userCountLimit = computed(() => {
-    let limit = PLANS.find((plan) => plan.type === currentPlan.value)
-      ?.maximumSeatCount ?? 0;
-    if (limit < 0) { limit = Number.MAX_VALUE; }
-    const seatCount = subscription.value?.seats ?? 0;
-    if (seatCount < 0) { return Number.MAX_VALUE; }
-    if (seatCount === 0) { return limit; }
-    return seatCount;
+    // VNP-LIC-001: Unlimited seats
+    return Number.MAX_VALUE;
   });
```

### 4.2.3 — Override Expiry & Trial

```diff
-  const isExpired = computed(() => {
-    if (!subscription.value || !subscription.value.expiresTime || isFreePlan.value) {
-      return false;
-    }
-    return dayjs(getDateForPbTimestampProtoEs(subscription.value.expiresTime))
-      .isBefore(new Date());
-  });
+  const isExpired = computed(() => false); // VNP-LIC-001: Never expired

-  const isTrialing = computed(() => !!subscription.value?.trialing);
+  const isTrialing = computed(() => false); // VNP-LIC-001

-  const showTrial = computed(() => {
-    if (!isSelfHostLicense.value) { return false; }
-    if (!subscription.value || isFreePlan.value) { return true; }
-    return false;
-  });
+  const showTrial = computed(() => false); // VNP-LIC-001

-  const isHAAllowed = computed(() => subscription.value?.ha ?? false);
+  const isHAAllowed = computed(() => true); // VNP-LIC-001
```

### 4.2.4 — Override Feature Actions

```diff
   const hasFeature = (feature: PlanFeature) => {
-    if (isExpired.value) { return false; }
-    return checkFeature(currentPlan.value, feature);
+    // VNP-LIC-001: All features enabled
+    return true;
   };

   const hasInstanceFeature = (
     feature: PlanFeature,
     instance: Instance | InstanceResource | undefined = undefined
   ) => {
-    if (currentPlan.value === PlanType.FREE) { return hasFeature(feature); }
-    if (!instance || !instanceLimitFeature.has(feature)) { return hasFeature(feature); }
-    return checkInstanceFeature(
-      currentPlan.value, feature,
-      hasUnifiedInstanceLicense.value || instance.activation
-    );
+    // VNP-LIC-001: All instance features enabled
+    return true;
   };

   const instanceMissingLicense = (
     feature: PlanFeature,
     instance: Instance | InstanceResource | undefined = undefined
   ) => {
-    if (!instanceLimitFeature.has(feature)) { return false; }
-    if (!instance) { return false; }
-    if (hasUnifiedInstanceLicense.value) { return false; }
-    return hasFeature(feature) && !instance.activation;
+    // VNP-LIC-001: No instance ever missing license
+    return false;
   };
```

## Notes
- `fetchSubscription()`, `uploadLicense()`, `pollSubscriptionUntil()` giữ nguyên — vẫn gọi server bình thường
- `daysBeforeExpire`, `expireAt` computed có thể giữ nguyên vì `isExpired = false` → banners không hiển thị
- Exported `hasFeature()` ở line 395 cũng sẽ auto-work vì gọi `store.hasFeature()`

## AC
- [ ] `currentPlan` computed = `PlanType.ENTERPRISE`
- [ ] `isFreePlan` = `false`, `isExpired` = `false`, `isTrialing` = `false`
- [ ] `showTrial` = `false`, `isHAAllowed` = `true`
- [ ] `instanceCountLimit` = `Number.MAX_VALUE`, `userCountLimit` = `Number.MAX_VALUE`
- [ ] `hasFeature(anyFeature)` = `true`
- [ ] `hasInstanceFeature(anyFeature, anyInstance)` = `true`
- [ ] `instanceMissingLicense(anyFeature, anyInstance)` = `false`
- [ ] Update `subscription.test.ts` expectations nếu có
