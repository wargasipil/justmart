import { useEffect, useMemo } from "react";
import { Box, ChakraProvider, defaultSystem } from "@chakra-ui/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import type { Decorator, Preview } from "@storybook/react";
import type { ReactNode } from "react";
import { z } from "zod";

import i18n from "../src/lib/i18n";
import { zodErrorMap } from "../src/lib/zodErrorMap";
import { AppToaster } from "../src/lib/toaster";
import { usePreferencesStore, type Locale, type Theme } from "../src/stores/preferences";

// Install the global Zod error map, exactly as main.tsx does after i18n init —
// without it every form story shows Zod's untranslated English defaults instead
// of the app's `validation.*` copy, which is the opposite of what the forms
// HARD RULE says the user ever sees.
z.setErrorMap(zodErrorMap);

/**
 * The app's provider stack, minus AuthProvider and the real router.
 *
 * AuthProvider is deliberately absent: it reads tokens from localStorage and
 * would kick off a `Me` request. No shared component consumes `useAuth()` —
 * role gating lives in the pages — so a story never needs it. Add a
 * per-story mock decorator if a future shared component starts reading it.
 */
function StoryProviders({
  theme,
  locale,
  initialEntries,
  children,
}: {
  theme: Theme;
  locale: Locale;
  initialEntries: string[];
  children: ReactNode;
}) {
  // One client per story render. A module-level singleton would leak cached
  // rows between stories and make a story's result depend on what you viewed
  // before it. `retry: false` so a story against a stopped backend fails fast.
  const client = useMemo(
    () =>
      new QueryClient({
        defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
      }),
    [],
  );

  // Theme is applied through the real store setter, not by writing the
  // attribute here: `setTheme` is what toggles `data-theme` + the `dark` class
  // on <html>, which is the platform mechanism Chakra v3 flips its semantic
  // tokens on. Bypassing it would drift from the app the moment that changes.
  useEffect(() => {
    usePreferencesStore.getState().setTheme(theme);
  }, [theme]);

  useEffect(() => {
    void i18n.changeLanguage(locale);
    usePreferencesStore.getState().setLocale(locale);
  }, [locale]);

  return (
    <QueryClientProvider client={client}>
      <ChakraProvider value={defaultSystem}>
        <MemoryRouter initialEntries={initialEntries}>
          {/* Explicit bg/fg so the canvas follows the theme toggle — Storybook
              paints its own white ground behind the story otherwise. */}
          <Box bg="bg" color="fg" minH="100vh" p={6}>
            {children}
          </Box>
          <AppToaster />
        </MemoryRouter>
      </ChakraProvider>
    </QueryClientProvider>
  );
}

const withProviders: Decorator = (Story, context) => (
  <StoryProviders
    theme={context.globals.theme as Theme}
    locale={context.globals.locale as Locale}
    // A story that cares where it "is" (RouteTabs, Breadcrumbs, an active nav
    // link) sets parameters.router.initialEntries.
    initialEntries={(context.parameters.router?.initialEntries as string[]) ?? ["/"]}
  >
    <Story />
  </StoryProviders>
);

const preview: Preview = {
  decorators: [withProviders],

  globalTypes: {
    theme: {
      description: "Chakra color mode (data-theme on <html>)",
      toolbar: {
        icon: "contrast",
        dynamicTitle: true,
        items: [
          { value: "light", title: "Light" },
          { value: "dark", title: "Dark" },
        ],
      },
    },
    locale: {
      description: "UI language",
      toolbar: {
        icon: "globe",
        dynamicTitle: true,
        items: [
          { value: "id", title: "Bahasa Indonesia" },
          { value: "en", title: "English" },
        ],
      },
    },
  },

  // `id` first, matching the app's i18n fallbackLng — a story should show what
  // the shop actually sees.
  initialGlobals: { theme: "light", locale: "id" },

  parameters: {
    // The decorator paints the surface with Chakra's own `bg` token, so
    // Storybook's backgrounds addon would just fight it.
    layout: "fullscreen",
    backgrounds: { disable: true },
    controls: {
      matchers: { color: /(background|color)$/i, date: /Date$/i },
    },
  },
};

export default preview;
