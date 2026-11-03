import { Button, Flex, HStack, Text } from "@chakra-ui/react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import EnumSelect from "./EnumSelect";
import { ALL_LIMIT, LOAD_MORE_STEP, PAGE_SIZE_OPTIONS } from "../lib/pagination";

// Shared list pagination control. Page is 0-based. Render it under any
// server-paginated table; pair it with usePageState (lib/pagination.ts).
//
// Two forms, switched purely by CSS breakpoint (like AppShell):
//   md+  — "Showing X–Y of N" + page-size select + Prev/Next.
//   phone — "Showing 1–Y of N" + a full-width "Load more", which keeps offset 0
//           and grows pageSize by LOAD_MORE_STEP so the loaded rows stay and
//           more are appended. The list's query must use keepPageData (every
//           List hook in queries/ does) or it blanks on each tap.
export type PaginationProps = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
  /**
   * The next rows are loading — pass the list query's `isPlaceholderData`.
   * Spins the Load more button so a tap on a slow connection isn't mistaken
   * for a dead button.
   */
  loading?: boolean;
};

export default function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  loading,
}: PaginationProps) {
  const { t } = useTranslation();
  const from = total === 0 ? 0 : page * pageSize + 1;
  const to = Math.min((page + 1) * pageSize, total);
  const canPrev = page > 0;
  const canNext = to < total;
  // The server clamps a page at ALL_LIMIT, so past that "more" can't grow.
  const canLoadMore = canNext && to < ALL_LIMIT;

  // Load more grows the page from the rows already shown; without a size
  // setter it degrades to stepping forward a page.
  const loadMore = () =>
    onPageSizeChange
      ? onPageSizeChange(Math.min(to + LOAD_MORE_STEP, ALL_LIMIT))
      : onPageChange(page + 1);

  // A grown Load-more size (50, 75…) isn't a preset; keep it selectable so the
  // select doesn't render blank after a phone session widens to desktop.
  const sizes = PAGE_SIZE_OPTIONS.includes(pageSize as (typeof PAGE_SIZE_OPTIONS)[number])
    ? [...PAGE_SIZE_OPTIONS]
    : [...PAGE_SIZE_OPTIONS, pageSize].sort((a, b) => a - b);

  return (
    // One "Showing" line for both forms (a second copy would double every
    // getByText match); only the controls switch. column-reverse puts the
    // phone's Load more button above the count.
    <Flex
      direction={{ base: "column-reverse", md: "row" }}
      justify="space-between"
      align={{ base: "stretch", md: "center" }}
      pt={1}
      gap={2}
      wrap={{ md: "wrap" }}
    >
      <Text fontSize="sm" color="fg.muted" textAlign={{ base: "center", md: "start" }}>
        {t("common.pagination.showing", { from, to, total })}
      </Text>
      <HStack hideBelow="md" gap={2} wrap="wrap">
        {onPageSizeChange && (
          <EnumSelect
            size="sm"
            width="32"
            value={String(pageSize)}
            onChange={(v) => onPageSizeChange(Number(v))}
            items={sizes.map((n) => String(n))}
            itemToString={(s) => t("common.pagination.perPage", { count: Number(s) })}
            itemToValue={(s) => s}
          />
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={!canPrev}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft size={16} />
          {t("common.pagination.prev")}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!canNext}
          onClick={() => onPageChange(page + 1)}
        >
          {t("common.pagination.next")}
          <ChevronRight size={16} />
        </Button>
      </HStack>
      {canLoadMore && (
        <Button
          hideFrom="md"
          size="sm"
          variant="outline"
          loading={loading}
          onClick={loadMore}
        >
          {t("common.pagination.loadMore")}
        </Button>
      )}
    </Flex>
  );
}
