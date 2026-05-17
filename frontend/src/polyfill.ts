(() => {
  if (typeof WeakRef === "undefined") {
    console.warn(
      "[Bytebase] Your browser does not support WeakRef. " +
      "Memory usage may increase over time. Please upgrade your browser."
    );
    class WeakRefShim<T extends WeakKey> {
      readonly [Symbol.toStringTag] = "WeakRef";
      private readonly _target: T;
      constructor(target: T) { this._target = target; }
      deref(): T | undefined { return this._target; }
    }

    (
      globalThis as typeof globalThis & { WeakRef: typeof WeakRefShim }
    ).WeakRef = WeakRefShim;
  }
})();
