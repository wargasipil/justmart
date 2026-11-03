import { Code, HStack, Text } from "@chakra-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import type { Decorator } from "@storybook/react";
import { useEffect, useMemo, type ReactNode } from "react";

import GlossaryBridge from "../../components/GlossaryBridge";
import { Role } from "../../gen/auth_iface/v1/policy_pb";
import { GetBussinessSettingsResponse, BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { User } from "../../gen/user_iface/v1/users_pb";
import { AuthContext, type AuthState } from "../../lib/auth";
import i18n from "../../lib/i18n";
import en from "../../locales/en.json";
import id from "../../locales/id.json";
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

// ---------------------------------------------------------------------------
// Page stories
// ---------------------------------------------------------------------------

/** The fixed user a page story is signed in as. Ids are fixture values. */
export const STORY_USERS: Record<"owner" | "admin" | "cashier" | "apoteker", User> = {
  owner: new User({ id: "user-owner", name: "Budi Santoso", email: "owner@toko.test", role: Role.OWNER, active: true }),
  admin: new User({ id: "user-admin", name: "Sari Wulandari", email: "admin@toko.test", role: Role.PHARMACIST, active: true }),
  cashier: new User({ id: "user-cashier", name: "Dewi Lestari", email: "kasir@toko.test", role: Role.CASHIER, active: true }),
  apoteker: new User({ id: "user-apoteker", name: "apt. Rina Kusuma", email: "apoteker@toko.test", role: Role.APOTEKER, active: true }),
};

function StoryAuth({ user, children }: { user: User; children: ReactNode }) {
  // A static session: no tokens, no `Me`. login/logout are inert because a
  // story never navigates to /login.
  const value = useMemo<AuthState>(
    () => ({
      user,
      loading: false,
      login: async () => {},
      logout: async () => {},
      refreshUser: async () => {},
    }),
    [user],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/**
 * GlossaryBridge rewrites the `glossary` i18n bundle IN PLACE (Produk → Obat),
 * and i18n is a module singleton shared by every story. Put the retail noun
 * back on unmount, or a component story opened after a pharmacy page story
 * would keep saying "Obat".
 */
function RestoreGlossaryOnUnmount() {
  useEffect(
    () => () => {
      i18n.addResourceBundle("en", "translation", { glossary: en.glossary }, true, true);
      i18n.addResourceBundle("id", "translation", { glossary: id.glossary }, true, true);
    },
    [],
  );
  return null;
}

export type PageContext = {
  /** Who is signed in. Default "owner". */
  user?: keyof typeof STORY_USERS;
  /** The shop's business mode. Default "retail". */
  mode?: "retail" | "pharmacy";
};

/**
 * The app context a routed page expects, minus the network: a signed-in user
 * of the given role, the shop's business mode, and the real GlossaryBridge
 * (mounted by App.tsx in the app, so a page rendered alone would never swap
 * the catalog noun). Data comes from MSW — see `storyMocks.ts`.
 *
 * Configured through `parameters.pageContext`, NOT by passing options to the
 * decorator: Storybook ADDS a story's decorators to the meta's rather than
 * replacing them, so a per-story `withPageContext("pharmacy")` would nest
 * inside the meta's retail one and the two GlossaryBridges would fight
 * (the outer one's effect runs last and wins). Parameters merge instead.
 *
 * ```ts
 * decorators: [withPageContext],
 * parameters: { pageContext: { user: "cashier", mode: "pharmacy" } },
 * ```
 */
export const withPageContext: Decorator = (Story, context) => {
  const { user = "owner", mode = "retail" } = (context.parameters.pageContext ?? {}) as PageContext;
  const value = mode === "pharmacy" ? BussinessType.PHARMACY_SHOP : BussinessType.RETAIL;
  return (
    <SeedBusinessMode mode={value}>
      <StoryAuth user={STORY_USERS[user]}>
        <GlossaryBridge />
        <RestoreGlossaryOnUnmount />
        <Story />
      </StoryAuth>
    </SeedBusinessMode>
  );
};
