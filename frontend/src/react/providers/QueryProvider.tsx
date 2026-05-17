// i18n: i18next | use t("key") from useTranslation()
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { ConnectError, Code } from "@connectrpc/connect";
import type { ReactNode } from "react";

/**
 * Shared QueryClient with ConnectRPC-aware retry logic.
 *
 * - staleTime: 5 min  → avoids needless re-fetches on tab focus
 * - gcTime:   30 min  → keeps inactive entries for back-navigation
 * - retry:    max 2 attempts, instant bail on auth / permission / 404 errors
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      gcTime: 30 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        // Never retry non-transient gRPC errors
        if (error instanceof ConnectError) {
          const noRetry: Code[] = [
            Code.NotFound,
            Code.PermissionDenied,
            Code.Unauthenticated,
            Code.InvalidArgument,
            Code.AlreadyExists,
            Code.FailedPrecondition,
          ];
          if (noRetry.includes(error.code)) return false;
        }
        return failureCount < 2;
      },
    },
    mutations: {
      retry: false,
    },
  },
});

/**
 * QueryProvider — wraps React tree with TanStack Query context.
 * DevTools are only loaded in development mode.
 */
export function QueryProvider({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      {import.meta.env.DEV && (
        <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
      )}
    </QueryClientProvider>
  );
}
