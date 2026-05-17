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
