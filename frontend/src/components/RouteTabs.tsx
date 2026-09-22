import { Tabs } from "@chakra-ui/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";

import EnumSelect from "./EnumSelect";
import { useTabsAsSelect, type TabsOrientation } from "../lib/tabs";

// URL-driven tabs that look and behave like Chakra v3's `Tabs.Root` (default
// `variant="line"` — underline on active). Each tab is a `react-router-dom`
// route under an `<Outlet/>` rendered by the parent page. The active state is
// derived from `useLocation()`; switching tabs calls `navigate()` on
// `onValueChange` so it's a client-side SPA route change (no full reload).
//
// NOTE: tabs are plain Chakra `Tabs.Trigger` buttons, NOT `<a href>` links.
// The earlier `<Tabs.Trigger asChild><NavLink/></Tabs.Trigger>` shape rendered
// an anchor whose native navigation won over NavLink's SPA handler, causing a
// full-page reload on every tab click. The controlled-navigate approach below
// is the documented fix. Tradeoff: no middle-click/"open in new tab" on a tab;
// deep-linking is unaffected (each tab still has its own URL).
//
// Below `md` it is neither a strip nor a rail but an <EnumSelect> of the same
// items (see `useTabsAsSelect`): a phone has no room for a row of tabs, and a
// strip that scrolls hides the ones that do not fit — including, since nothing
// scrolls the active trigger into view, the selected one. A picker cannot clip
// an option and always names the current tab.
//
// Use this for page-level tab navigation where each panel has its own URL
// (Analytics, Inventory, Purchasing). For state-driven tabs that share one
// route (Tax: Issued invoices / NSFP pool), use Chakra `Tabs.Root` directly.
// Codified in CLAUDE.md → Frontend conventions → Tabs.
export type RouteTabItem = {
  /** Stable identifier for Chakra Tabs internals. Usually the last URL segment. */
  value: string;
  /** Already-localized label (caller passes `t("...")` result). */
  label: string;
  /** Absolute path the tab navigates to. */
  to: string;
};

export type RouteTabsProps = {
  items: RouteTabItem[];
  /**
   * `"vertical"` renders a left-hand rail instead of a strip; the caller then
   * lays the `<Outlet/>` out BESIDE it (see `routes/Settings.tsx`). Resolve it
   * with `useTabsOrientation()` rather than hard-coding `"vertical"`, so the
   * rail degrades to a strip on a narrow viewport where it would not fit.
   * Defaults to the horizontal strip.
   */
  orientation?: TabsOrientation;
};

export default function RouteTabs({ items, orientation = "horizontal" }: RouteTabsProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const vertical = orientation === "vertical";
  const asSelect = useTabsAsSelect();

  // EnumSelect memoizes its collection on the `items` IDENTITY, and every
  // caller builds this array inline each render (the labels are `t(...)`
  // calls), so passing it straight through would rebuild the collection on
  // every paint — which is the spin its own comment warns about. Key on the
  // CONTENT instead: a locale change alters the labels and correctly yields a
  // new array; a re-render alone does not.
  const itemsKey = items.map((it) => `${it.value}\u0000${it.label}\u0000${it.to}`).join("\u0001");
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const options = useMemo(() => items, [itemsKey]);

  // Pick the tab whose `to` is the LONGEST prefix of the current pathname.
  // This avoids the wrong tab lighting up when one tab's path is itself a
  // prefix of another's (or of an unrelated sub-route). Falls back to the
  // first item if nothing matches.
  const activeValue =
    items
      .filter((it) => location.pathname.startsWith(it.to))
      .sort((a, b) => b.to.length - a.to.length)[0]?.value ?? items[0]?.value;

  if (asSelect) {
    return (
      <EnumSelect
        width="full"
        // The picker IS this page's navigation here, so it needs a name: the
        // desktop form is a tablist whose triggers read as tabs, while an
        // unnamed combobox announces only its current value.
        ariaLabel={t("common.section")}
        value={activeValue ?? null}
        onChange={(v) => {
          const to = items.find((it) => it.value === v)?.to;
          if (to && v !== activeValue) navigate(to);
        }}
        items={options}
        itemToString={(it) => it.label}
        itemToValue={(it) => it.value}
      />
    );
  }

  return (
    <Tabs.Root
      orientation={orientation}
      value={activeValue}
      variant="line"
      onValueChange={(d) => {
        const to = items.find((it) => it.value === d.value)?.to;
        // Guard against a redundant navigate when the value just synced to the
        // already-active tab.
        if (to && d.value !== activeValue) navigate(to);
      }}
    >
      {/* Triggers never shrink: squeezing them overlaps the labels (a strip of
          8 status tabs, or one holding "Default warehouse", does not fit a
          phone). The strip scrolls instead. A vertical rail is a fixed-width
          column, so it neither shrinks nor scrolls. */}
      <Tabs.List
        minW={vertical ? "180px" : undefined}
        flexShrink={0}
        overflowX={vertical ? undefined : "auto"}
        // overflow-y MUST be pinned whenever overflow-x is set: per CSS spec a
        // `visible` value on one axis computes to `auto` when the other axis is
        // not visible. The active trigger's underline makes the content a hair
        // taller than the list, so that implicit `overflow-y: auto` rendered a
        // stray vertical scrollbar next to the strip (arrows and all) even when
        // every tab fit on screen.
        overflowY={vertical ? undefined : "hidden"}
      >
        {items.map((it) => (
          <Tabs.Trigger
            key={it.value}
            value={it.value}
            flexShrink={0}
            justifyContent={vertical ? "flex-start" : undefined}
            w={vertical ? "full" : undefined}
          >
            {it.label}
          </Tabs.Trigger>
        ))}
      </Tabs.List>
    </Tabs.Root>
  );
}
