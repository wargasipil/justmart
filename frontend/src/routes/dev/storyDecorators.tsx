import { Code, HStack, Text } from "@chakra-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import type { Decorator } from "@storybook/react";
import type { ReactNode } from "react";

import { GetBussinessSettingsResponse, BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { settingsKeys } from "../../queries/settings";

// Story-scoped decorators + the small preview helpers stories share.
//
// These live beside the gallery registry rather than in `.storybook/` because
// they are app knowledge (which query key holds what), not Storybook plumbing.
// Storybook applies story decorators INSIDE the global ones, so anything here
// can reach the QueryClient the preview decorator mounted.

/**
 * Readout of what a controlled component just emitted.
 *
 * Several inputs here have a deliberately non-obvious emit contract —
 * `MoneyInput` hands back an unformatted digit string, `""` for zero — so
 * showing it turns the preview into a contract check rather than a picture.
 */
export function Emitted({ children }: { children: string }) {
  return (
    <HStack gap={2}>
      <Text fontSize="xs" color="fg.muted">
        onChange →
      </Text>
      <Code size="sm">{children === "" ? '""' : children}</Code>
    </HStack>
  );
}

function SeedBusinessMode({ mode, children }: { mode: BussinessType; children: ReactNode }) {
  const qc = useQueryClient();
  // Seeded during render, before the child's useQuery subscribes — so the
  // component reads the mode synchronously and never fires the RPC. Doing this
  // in an effect would let one render slip through in the wrong mode.
  qc.setQueryData(
    settingsKeys.businessMode,
    new GetBussinessSettingsResponse({ type: mode, appTitle: "" }),
  );
  return <>{children}</>;
}

/**
 * Pin the shop's business mode for a story, without a backend.
 *
 * Several shared components are mode-aware — the catalog noun swaps
 * Produk ↔ Obat, the Rx affordances appear — and that behavior is invisible
 * unless a story can choose the mode.
 *
 * ```ts
 * export const Pharmacy: Story = { decorators: [withBusinessMode("pharmacy")] };
 * ```
 */
export function withBusinessMode(mode: "retail" | "pharmacy"): Decorator {
  const value = mode === "pharmacy" ? BussinessType.PHARMACY_SHOP : BussinessType.RETAIL;
  return (Story) => (
    <SeedBusinessMode mode={value}>
      <Story />
    </SeedBusinessMode>
  );
}
