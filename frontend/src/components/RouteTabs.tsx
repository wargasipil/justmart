import { Tabs } from "@chakra-ui/react";
import { useLocation, useNavigate } from "react-router-dom";

import type { TabsOrientation } from "../lib/tabs";

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
  const location = useLocation();
  const navigate = useNavigate();
  const vertical = orientation === "vertical";

  // Pick the tab whose `to` is the LONGEST prefix of the current pathname.
  // This avoids the wrong tab lighting up when one tab's path is itself a
  // prefix of another's (or of an unrelated sub-route). Falls back to the
  // first item if nothing matches.
  const activeValue =
    items
      .filter((it) => location.pathname.startsWith(it.to))
      .sort((a, b) => b.to.length - a.to.length)[0]?.value ?? items[0]?.value;

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
