// AI: Copy + rename for filter/sort/pagination hooks.
// Pattern: useState + URL search params sync
//
// RULES:
//   1. Filters stored in URL search params for shareable links
//   2. Default values when param is missing
//   3. Return setters alongside values for parent to use

import { useState, useMemo, useCallback } from "react";

// TODO: Import URL sync utilities if available
// import { useSearchParams } from "react-router-dom";  // or custom hook

interface FilterState {
  /** Text search query */
  search: string;
  /** Sort field */
  sortBy: string;
  /** Sort direction */
  sortOrder: "asc" | "desc";
  /** Current page (0-indexed) */
  page: number;
  /** Items per page */
  pageSize: number;
}

const DEFAULT_FILTERS: FilterState = {
  search: "",
  sortBy: "name",
  sortOrder: "asc",
  page: 0,
  pageSize: 50,
};

/**
 * Filter/sort/pagination state hook.
 * Usage: const { filters, setSearch, setSortBy, resetFilters } = useTemplateFilters();
 */
export function useTemplateFilters(initialOverrides?: Partial<FilterState>) {
  const [filters, setFilters] = useState<FilterState>({
    ...DEFAULT_FILTERS,
    ...initialOverrides,
  });

  const setSearch = useCallback((search: string) => {
    setFilters((prev) => ({ ...prev, search, page: 0 })); // Reset page on search
  }, []);

  const setSortBy = useCallback((sortBy: string) => {
    setFilters((prev) => ({
      ...prev,
      sortBy,
      sortOrder: prev.sortBy === sortBy && prev.sortOrder === "asc" ? "desc" : "asc",
    }));
  }, []);

  const setPage = useCallback((page: number) => {
    setFilters((prev) => ({ ...prev, page }));
  }, []);

  const setPageSize = useCallback((pageSize: number) => {
    setFilters((prev) => ({ ...prev, pageSize, page: 0 }));
  }, []);

  const resetFilters = useCallback(() => {
    setFilters({ ...DEFAULT_FILTERS, ...initialOverrides });
  }, [initialOverrides]);

  // Build ConnectRPC filter string from state
  const filterString = useMemo(() => {
    const parts: string[] = [];
    if (filters.search) {
      parts.push(`title.contains("${filters.search}")`);
    }
    // TODO: Add more filter expressions as needed
    return parts.join(" && ");
  }, [filters.search]);

  return {
    filters,
    filterString,
    setSearch,
    setSortBy,
    setPage,
    setPageSize,
    resetFilters,
  };
}
