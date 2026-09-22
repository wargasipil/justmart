import { useBreakpointValue } from "@chakra-ui/react";

export type TabsOrientation = "horizontal" | "vertical";

/**
 * Resolves a tab strip's orientation against the viewport: a vertical rail on
 * `md`+, an ordinary horizontal strip below, where there is no room for a rail
 * to sit beside its panel.
 *
 * This is the one place in the app that reaches for a JS breakpoint hook
 * instead of the usual CSS breakpoint props (`display={{ base, md }}`, as in
 * TopBar). Ark's `orientation` is a behavioural prop, not a style prop, so an
 * object value never reaches it — the decision has to be made in JS. Callers
 * then drive their own layout (a `Flex` direction, say) off the returned value
 * rather than a second `{{ base, md }}`, so the breakpoint is stated once.
 *
 * `ssr: false` resolves the media query on the very first render, so a desktop
 * user never sees a horizontal flash — this is a pure SPA, there is no server
 * render to mismatch against.
 */
export function useTabsOrientation(): TabsOrientation {
  return (
    useBreakpointValue<TabsOrientation>({ base: "horizontal", md: "vertical" }, { ssr: false }) ??
    "vertical"
  );
}

/**
 * Whether a tab strip has to become a picker instead of a strip.
 *
 * Below `md` a row of tabs does not fit: at 390px `/settings` renders 573px of
 * triggers into a 366px strip, so two of its six tabs sit entirely off screen,
 * a third is cut mid-word, and — because nothing scrolls the active trigger
 * into view — opening `/settings/backups` shows a strip with no tab selected
 * at all. Touch scrollbars are invisible at rest, so there is not even a hint
 * that the rest exists. `/purchasing` is worse (8 tabs).
 *
 * `<RouteTabs>` therefore renders the same items as an `<EnumSelect>` there:
 * picking one of a short fixed set is exactly what that wrapper is for (the
 * Selects HARD RULE), it cannot clip or hide an option, and the trigger always
 * names where you are. Same breakpoint as `useTabsOrientation`, so the two
 * steps — rail → strip → picker — are stated once each.
 */
export function useTabsAsSelect(): boolean {
  return useBreakpointValue({ base: true, md: false }, { ssr: false }) ?? false;
}
