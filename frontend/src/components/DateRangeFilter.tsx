import { useEffect, useRef, useState } from "react";
import {
  Box,
  Button,
  DatePicker,
  Field,
  Flex,
  HStack,
  Input,
  Popover,
  Portal,
  SegmentGroup,
  Stack,
  Text,
  parseDate,
} from "@chakra-ui/react";
import { Check, ChevronDown, Clock } from "lucide-react";
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
  timePartOf,
  type DateRange,
  type RangePreset,
} from "../lib/dateRange";

// Grafana-style time-range picker — a single control:
//
//   [ 🕐 Last 30 days ▾ ]            (no `fields`)
//   [ 🕐 Created · Last 30 days ▾ ]  (with `fields`)
//
// It opens a popover holding, top to bottom: an optional **date-field** row
// ("which date am I filtering on?"), then two panes — an **absolute** range on
// the left (From / To text inputs down to the second + an inline range
// calendar) and the searchable **quick ranges** list on the right.
//
// The range model + all the date math live in lib/dateRange.ts; this file is
// only the UI. Callers keep a `DateRange` in state and read `fromUnix`/`toUnix`.

/** One selectable date column, e.g. `{ value: "received", label: "Arrived" }`. */
export type DateFieldOption = { value: string; label: string };

type Props = {
  value: DateRange;
  onChange: (next: DateRange) => void;
  size?: "xs" | "sm" | "md";
  /**
   * Which date column the range applies to. Omit entirely on a surface with
   * only one filterable date (Orders, analytics) — the row then isn't rendered.
   * When given, an "Any date" entry is prepended: it's how the user turns the
   * date filter OFF, and it reports back as `field: ""`.
   */
  fields?: readonly DateFieldOption[];
  /** Selected field value. `""` = Any date (send no from/to to the RPC). */
  field?: string;
  onFieldChange?: (field: string) => void;
};

const DEFAULT_FROM_TIME = "00:00:00";
const DEFAULT_TO_TIME = "23:59:59";

// SegmentGroup treats "" as "nothing selected", so Any date rides an internal
// sentinel. The PUBLIC value stays "" — that's what the RPCs want for `off`.
const ANY_FIELD = "__any";

export default function DateRangeFilter({
  value,
  onChange,
  size = "sm",
  fields,
  field = "",
  onFieldChange,
}: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const hasFields = !!fields?.length;
  const anyDate = hasFields && !field;
  const fieldLabel = fields?.find((f) => f.value === field)?.label;
  // "Any date" replaces the range in the label rather than qualifying it: with
  // no date column selected there is no range in effect to name.
  const label = anyDate
    ? t("common.anyDate")
    : fieldLabel
    ? `${fieldLabel} · ${rangeLabel(value, t)}`
    : rangeLabel(value, t);

  return (
    /* lazyMount + unmountOnExit so the panes mount fresh on every open: the
       From/To draft state is seeded from the CURRENT range instead of showing
       whatever was typed last time (Chakra keeps closed content mounted). */
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
              {hasFields && (
                <HStack gap={3} px={4} py={3} borderBottomWidth="1px" flexWrap="wrap">
                  <Text fontSize="xs" color="fg.muted" flexShrink={0}>
                    {t("analytics.range.picker.fieldLabel")}
                  </Text>
                  {/* A segmented control, not a <EnumSelect>: a nested select
                      popover inside this popover fights it for outside-click. */}
                  <SegmentGroup.Root
                    size="sm"
                    value={field || ANY_FIELD}
                    onValueChange={(e) => {
                      const next = e.value === ANY_FIELD ? "" : e.value ?? "";
                      onFieldChange?.(next);
                      // Any date is a terminal choice — there's no range left to
                      // pick, so don't leave the user staring at dimmed panes.
                      if (!next) setOpen(false);
                    }}
                  >
                    <SegmentGroup.Indicator />
                    <SegmentGroup.Item value={ANY_FIELD}>
                      <SegmentGroup.ItemText>{t("common.anyDate")}</SegmentGroup.ItemText>
                      <SegmentGroup.ItemHiddenInput />
                    </SegmentGroup.Item>
                    {fields!.map((f) => (
                      <SegmentGroup.Item key={f.value} value={f.value}>
                        <SegmentGroup.ItemText>{f.label}</SegmentGroup.ItemText>
                        <SegmentGroup.ItemHiddenInput />
                      </SegmentGroup.Item>
                    ))}
                  </SegmentGroup.Root>
                </HStack>
              )}
              <RangePanes
                value={value}
                disabled={anyDate}
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
  );
}

// The popover body: absolute pane + quick-range list. Split out so its draft
// state (the From/To text being typed) is created fresh on each open and thrown
// away on close — nothing half-typed can leak into the applied range.
function RangePanes({
  value,
  disabled = false,
  onApply,
}: {
  value: DateRange;
  /** True while the field row sits on "Any date" — there's no range to pick. */
  disabled?: boolean;
  onApply: (next: DateRange) => void;
}) {
  const { t, i18n } = useTranslation();
  const bounds = rangeBounds(value);
  const [fromText, setFromText] = useState(() => formatAbsolute(bounds.from));
  const [toText, setToText] = useState(() => formatAbsolute(bounds.to));

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

  const apply = () => {
    if (fromDate && toDate && !badOrder) onApply(absoluteRange(fromDate, toDate));
  };

  // On "Any date" there is no range in effect, so the panes would be a set of
  // controls that change nothing. Show the next step instead of dimming them.
  if (disabled) {
    return (
      <Box p={4} maxW="330px">
        <Text fontSize="sm" color="fg.muted">
          {t("analytics.range.picker.pickFieldFirst")}
        </Text>
      </Box>
    );
  }

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
      <QuickRangePane active={value.preset} onPick={onApply} />
    </Flex>
  );
}

// The right-hand pane: search box + the quick-range list. Owns its own search
// state and shares nothing with the absolute pane, so it lives on its own.
function QuickRangePane({
  active,
  onPick,
}: {
  active: RangePreset;
  onPick: (next: DateRange) => void;
}) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);

  // Grafana focuses the quick-range search on open, so typing "30" then Enter
  // is the whole interaction for the common case.
  useEffect(() => {
    searchRef.current?.focus();
  }, []);

  const needle = search.trim().toLowerCase();
  const quick = QUICK_RANGES.map((preset) => ({
    preset,
    label: t(`analytics.range.${preset}`),
  })).filter((q) => !needle || q.label.toLowerCase().includes(needle));

  return (
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
            onPick(resolveRange(quick[0].preset));
          }
        }}
      />
      <Stack gap={0} maxH="460px" overflowY="auto">
        {quick.map((q) => {
          const isActive = active === q.preset;
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
              bg={isActive ? "bg.muted" : "transparent"}
              color={isActive ? "colorPalette.solid" : "fg"}
              colorPalette="blue"
              _hover={{ bg: "bg.muted" }}
              cursor="pointer"
              onClick={() => onPick(resolveRange(q.preset))}
            >
              <Text truncate>{q.label}</Text>
              {isActive && <Check size={14} />}
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
  );
}
