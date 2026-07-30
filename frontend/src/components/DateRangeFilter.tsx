import { useEffect, useRef, useState } from "react";
import {
  Button,
  DatePicker,
  Field,
  Flex,
  HStack,
  IconButton,
  Input,
  Popover,
  Portal,
  Stack,
  Text,
  parseDate,
} from "@chakra-ui/react";
import { Check, ChevronDown, ChevronLeft, ChevronRight, Clock, ZoomOut } from "lucide-react";
import { useTranslation } from "react-i18next";

import { CalendarViews } from "./DatePicker";
import {
  QUICK_RANGES,
  absoluteRange,
  formatAbsolute,
  formatDateOnly,
  parseAbsolute,
  rangeBounds,
  rangeLabel,
  resolveRange,
  shiftRange,
  timePartOf,
  zoomOutRange,
  type DateRange,
} from "../lib/dateRange";

// Grafana-style time-range picker. One control group:
//
//   [ ◀ ] [ 🕐 Last 30 days ▾ ] [ ▶ ] [ 🔍− ]
//
// - the ◀ / ▶ arrows shift the window by its own width,
// - 🔍− zooms out 2× around the midpoint,
// - the middle button opens a two-pane popover: an **absolute** range on the
//   left (From / To text inputs down to the second + an inline range calendar)
//   and the searchable **quick ranges** list on the right.
//
// The range model + all the date math live in lib/dateRange.ts; this file is
// only the UI. Callers keep a `DateRange` in state and read `fromUnix`/`toUnix`.
type Props = {
  value: DateRange;
  onChange: (next: DateRange) => void;
  size?: "xs" | "sm" | "md";
};

const DEFAULT_FROM_TIME = "00:00:00";
const DEFAULT_TO_TIME = "23:59:59";

export default function DateRangeFilter({ value, onChange, size = "sm" }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const label = rangeLabel(value, t);

  return (
    <HStack gap={1}>
      <IconButton
        aria-label={t("analytics.range.picker.shiftBack")}
        title={t("analytics.range.picker.shiftBack")}
        variant="outline"
        size={size}
        onClick={() => onChange(shiftRange(value, -1))}
      >
        <ChevronLeft size={16} />
      </IconButton>

      {/* lazyMount + unmountOnExit so the panes mount fresh on every open: the
          From/To draft state is seeded from the CURRENT range instead of showing
          whatever was typed last time (Chakra keeps closed content mounted). */}
      <Popover.Root
        open={open}
        onOpenChange={(d) => setOpen(d.open)}
        // bottom-start: the trigger usually sits at the LEFT of a toolbar, so the
        // panes open rightward instead of overhanging the sidebar (floating-ui
        // still flips/shifts it when there isn't room).
        positioning={{ placement: "bottom-start" }}
        lazyMount
        unmountOnExit
      >
        <Popover.Trigger asChild>
          <Button variant="outline" size={size} fontWeight="normal" title={label} maxW="340px">
            <Clock size={14} />
            <Text truncate>{label}</Text>
            <ChevronDown size={14} />
          </Button>
        </Popover.Trigger>
        <Portal>
          <Popover.Positioner>
            <Popover.Content width="auto" maxW="min(96vw, 660px)">
              <Popover.Body p={0}>
                <RangePanes
                  value={value}
                  onApply={(next) => {
                    onChange(next);
                    setOpen(false);
                  }}
                />
              </Popover.Body>
            </Popover.Content>
          </Popover.Positioner>
        </Portal>
      </Popover.Root>

      <IconButton
        aria-label={t("analytics.range.picker.shiftForward")}
        title={t("analytics.range.picker.shiftForward")}
        variant="outline"
        size={size}
        onClick={() => onChange(shiftRange(value, 1))}
      >
        <ChevronRight size={16} />
      </IconButton>
      <IconButton
        aria-label={t("analytics.range.picker.zoomOut")}
        title={t("analytics.range.picker.zoomOut")}
        variant="outline"
        size={size}
        onClick={() => onChange(zoomOutRange(value))}
      >
        <ZoomOut size={16} />
      </IconButton>
    </HStack>
  );
}

// The popover body: absolute pane + quick-range list. Split out so its draft
// state (the From/To text being typed) is created fresh on each open and thrown
// away on close — nothing half-typed can leak into the applied range.
function RangePanes({
  value,
  onApply,
}: {
  value: DateRange;
  onApply: (next: DateRange) => void;
}) {
  const { t, i18n } = useTranslation();
  const bounds = rangeBounds(value);
  const [fromText, setFromText] = useState(() => formatAbsolute(bounds.from));
  const [toText, setToText] = useState(() => formatAbsolute(bounds.to));
  const [search, setSearch] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);

  // Grafana focuses the quick-range search on open, so typing "30" then Enter
  // is the whole interaction for the common case.
  useEffect(() => {
    searchRef.current?.focus();
  }, []);

  const fromDate = parseAbsolute(fromText);
  const toDate = parseAbsolute(toText);
  const badFormat = (!fromDate && fromText.trim() !== "") || (!toDate && toText.trim() !== "");
  const badOrder = !!fromDate && !!toDate && fromDate.getTime() >= toDate.getTime();
  const canApply = !!fromDate && !!toDate && !badOrder;
  const error = badFormat
    ? t("analytics.range.picker.invalidFormat")
    : badOrder
    ? t("analytics.range.picker.invalidOrder")
    : undefined;

  // Calendar selection mirrors whatever the two inputs currently parse to; it
  // only ever rewrites the DAY, keeping the typed time-of-day intact.
  const calendarValue = [fromDate, toDate]
    .filter((d): d is Date => !!d)
    .map((d) => parseDate(formatDateOnly(d)));

  const needle = search.trim().toLowerCase();
  const quick = QUICK_RANGES.map((preset) => ({
    preset,
    label: t(`analytics.range.${preset}`),
  })).filter((q) => !needle || q.label.toLowerCase().includes(needle));

  const apply = () => {
    if (fromDate && toDate && !badOrder) onApply(absoluteRange(fromDate, toDate));
  };

  return (
    <Flex align="stretch" direction={{ base: "column", md: "row" }}>
      {/* --- Absolute pane -------------------------------------------------- */}
      <Stack gap={3} p={4} minW={{ md: "330px" }}>
        <Text fontSize="sm" fontWeight="medium">
          {t("analytics.range.picker.absoluteTitle")}
        </Text>

        <Field.Root invalid={!!error}>
          <Field.Label fontSize="xs">{t("analytics.range.picker.from")}</Field.Label>
          <Input
            size="sm"
            fontFamily="mono"
            value={fromText}
            placeholder="YYYY-MM-DD HH:mm:ss"
            onChange={(e) => setFromText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && apply()}
          />
        </Field.Root>
        <Field.Root invalid={!!error}>
          <Field.Label fontSize="xs">{t("analytics.range.picker.to")}</Field.Label>
          <Input
            size="sm"
            fontFamily="mono"
            value={toText}
            placeholder="YYYY-MM-DD HH:mm:ss"
            onChange={(e) => setToText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && apply()}
          />
          {error && <Field.ErrorText>{error}</Field.ErrorText>}
        </Field.Root>

        {/* Inline (non-popover) range calendar — reuses the app's one calendar
            implementation. Inline mode means no nested popover to fight with. */}
        <DatePicker.Root
          inline
          selectionMode="range"
          locale={i18n.language}
          value={calendarValue}
          // Open on the month the range starts in, not on today. Uncontrolled
          // afterwards, so the user can still page months freely.
          defaultFocusedValue={calendarValue[0]}
          onValueChange={(d) => {
            const [start, end] = d.value;
            if (start) {
              setFromText(`${start.toString()} ${timePartOf(fromText, DEFAULT_FROM_TIME)}`);
            }
            if (end) {
              setToText(`${end.toString()} ${timePartOf(toText, DEFAULT_TO_TIME)}`);
            }
          }}
        >
          <DatePicker.Content borderWidth="1px" borderRadius="md" boxShadow="none" p={2}>
            <CalendarViews />
          </DatePicker.Content>
        </DatePicker.Root>

        <Button size="sm" colorPalette="blue" disabled={!canApply} onClick={apply}>
          {t("analytics.range.picker.apply")}
        </Button>
      </Stack>

      {/* --- Quick ranges --------------------------------------------------- */}
      <Stack
        gap={2}
        p={4}
        minW={{ md: "240px" }}
        borderStartWidth={{ md: "1px" }}
        borderTopWidth={{ base: "1px", md: "0" }}
      >
        <Input
          ref={searchRef}
          size="sm"
          placeholder={t("analytics.range.picker.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => {
            // Enter picks the only remaining match — "30" + Enter = Last 30 days.
            if (e.key === "Enter" && quick.length > 0) {
              onApply(resolveRange(quick[0].preset));
            }
          }}
        />
        <Stack gap={0} maxH="460px" overflowY="auto">
          {quick.map((q) => {
            const active = value.preset === q.preset;
            return (
              <HStack
                as="button"
                key={q.preset}
                justify="space-between"
                textAlign="start"
                px={2}
                py={1.5}
                borderRadius="md"
                fontSize="sm"
                bg={active ? "bg.muted" : "transparent"}
                color={active ? "colorPalette.solid" : "fg"}
                colorPalette="blue"
                _hover={{ bg: "bg.muted" }}
                cursor="pointer"
                onClick={() => onApply(resolveRange(q.preset))}
              >
                <Text truncate>{q.label}</Text>
                {active && <Check size={14} />}
              </HStack>
            );
          })}
          {quick.length === 0 && (
            <Text fontSize="sm" color="fg.muted" py={2}>
              {t("common.noResults")}
            </Text>
          )}
        </Stack>
      </Stack>
    </Flex>
  );
}
