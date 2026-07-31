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
