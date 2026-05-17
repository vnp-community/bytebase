# TASK-LIC-004: Suppress License Banners

> **Source**: SOL-LIC-001 §4.4 | **Priority**: P1 | **Effort**: 15 min  
> **Status**: DONE | **Deps**: LIC-002, LIC-003

## Scope
- **EDIT** `src/react/components/BannersWrapper.tsx`

## What
Suppress trial banner, subscription expiry banner, và upgrade subscription banner trong `BannersWrapper`. Giữ nguyên demo banner, external URL banner, và announcement banner.

## Implementation

```diff
 export function BannersWrapper() {
-  const { serverInfo, needConfigureExternalUrl } = useServerState();
-  const { currentPlan, daysBeforeExpire, isExpired, isTrialing } =
-    useSubscriptionState();
-
-  const shouldShowSubscriptionBanner =
-    isExpired ||
-    isTrialing ||
-    (currentPlan !== PlanType.FREE &&
-      daysBeforeExpire <= LICENSE_EXPIRATION_THRESHOLD);
+  const { serverInfo, needConfigureExternalUrl } = useServerState();
+
+  // VNP-LIC-001: Suppress all license-related banners
+  const shouldShowSubscriptionBanner = false;
   const shouldShowExternalUrlBanner = !isDev() && needConfigureExternalUrl;

   return (
     <>
-      <BannerUpgradeSubscription />
+      {/* VNP-LIC-001: Disabled upgrade banner */}
       {serverInfo?.demo ? <BannerDemo /> : null}
       {shouldShowSubscriptionBanner ? <BannerSubscription /> : null}
       {shouldShowExternalUrlBanner ? <BannerExternalUrl /> : null}
       <BannerAnnouncement />
     </>
   );
 }
```

## Notes
- `BannerUpgradeSubscription` bị loại bỏ hoàn toàn (component này hiển thị khi server reports unlicensed features)
- `BannerSubscription` vẫn giữ nhưng condition = `false` → không render
- `BannerAnnouncement` vẫn hoạt động bình thường (Enterprise feature `FEATURE_DASHBOARD_ANNOUNCEMENT` đã được unlock bởi LIC-001)
- Cleanup unused imports: `useSubscriptionState`, `PlanType`, `LICENSE_EXPIRATION_THRESHOLD` có thể remove nhưng giữ lại cho dễ rollback

## AC
- [ ] `BannerUpgradeSubscription` không được render
- [ ] `BannerSubscription` không hiển thị (condition = `false`)
- [ ] `BannerDemo` vẫn hiển thị khi `serverInfo.demo = true`
- [ ] `BannerExternalUrl` vẫn hiển thị khi cần configure
- [ ] `BannerAnnouncement` vẫn hoạt động
- [ ] Update `BannersWrapper.test.tsx` expectations nếu có
