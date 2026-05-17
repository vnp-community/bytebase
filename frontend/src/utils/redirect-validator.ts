export function isValidRedirectUrl(url: string): boolean {
  if (!url || typeof url !== "string") return false;
  if (!url.startsWith("/")) return false;
  if (url.startsWith("//")) return false;
  const decoded = decodeURIComponent(url);
  if (decoded.startsWith("//")) return false;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) return false;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(decoded)) return false;
  if (url.includes("\0") || url.includes("%00")) return false;
  return true;
}

export function sanitizeRedirectUrl(url: string | undefined | null): string {
  if (!url || !isValidRedirectUrl(url)) return "/";
  return url;
}

export function validateRedirectUrl(url: string | undefined | null): string {
  if (!url || typeof url !== "string") return "/";
  const trimmed = url.trim();
  if (!trimmed.startsWith("/")) return "/";
  if (trimmed.startsWith("//")) return "/";
  if (/^\/[^/]/.test(trimmed) === false) return "/";
  if (trimmed.includes("\\")) return "/";
  return trimmed;
}
