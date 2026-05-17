# TASK-L-020: LocaleManager Singleton

> **Source**: SOL-LIM-007 §1.1 | **Priority**: P3 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/localeManager.ts`

## What
Tạo framework-agnostic locale manager singleton. Pub/sub pattern cho bidirectional sync giữa vue-i18n và i18next.

## Implementation

```typescript
// src/localeManager.ts — NEW
type LocaleChangeCallback = (locale: string) => void | Promise<void>;

class LocaleManager {
  private _locale: string = "en-US";
  private _subscribers = new Set<LocaleChangeCallback>();

  get locale(): string { return this._locale; }

  subscribe(callback: LocaleChangeCallback): () => void {
    this._subscribers.add(callback);
    return () => this._subscribers.delete(callback);
  }

  async setLocale(newLocale: string): Promise<void> {
    if (this._locale === newLocale) return;
    this._locale = newLocale;
    const promises = Array.from(this._subscribers).map((cb) => {
      try { return cb(newLocale); } catch { return undefined; }
    });
    await Promise.all(promises);
  }
}

export const localeManager = new LocaleManager();
```

## AC
- [ ] `localeManager.setLocale("vi-VN")` notifies all subscribers
- [ ] Duplicate locale set (same value) is no-op
- [ ] Subscriber error doesn't block other subscribers
- [ ] `subscribe()` returns unsubscribe function
