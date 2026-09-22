import { Code } from "@connectrpc/connect";
import { Box, Text } from "@chakra-ui/react";
import type { Decorator, StoryObj } from "@storybook/react";
import { useMemo, useState } from "react";
import { Route } from "react-router-dom";
import { expect, userEvent, within } from "storybook/test";

import { BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { SettingsService } from "../../gen/settings_iface/v1/settings_connect";
import { AuthService } from "../../gen/user_iface/v1/auth_connect";
import type { User } from "../../gen/user_iface/v1/users_pb";
import { AuthContext, type AuthState } from "../../lib/auth";
import { authClient } from "../../lib/clients";
import { STORY_USERS } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import Login from "../../routes/Login";

// /login — the one page rendered bare (App.tsx skips the AppShell for it), so
// it is shown as a page on its own rather than inside the shell.
//
// withPageContext always signs a user in, and Login redirects a signed-in
// user away — so this scenario brings its own SIGNED-OUT session. Its `login`
// makes the real AuthService.Login call (answered by MSW) and, on success,
// signs the returned user in locally: no tokens are written, so a story can't
// leave a fake session in localStorage for the next one. A wrong password is
// therefore a genuine Connect error reaching the page's own error toast.
//
// The brand (icon + heading) comes from the PUBLIC GetBranding, exactly as
// before sign-in in the app — not from the business-mode seed page stories use.
//
// Rendered by screens/mobile/pages/users/Login.stories.tsx.

function SignedOutAuth({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const value = useMemo<AuthState>(
    () => ({
      user,
      loading: false,
      login: async (email, password) => {
        const res = await authClient.login({ email, password });
        setUser(res.user ?? null);
      },
      logout: async () => setUser(null),
      refreshUser: async () => {},
    }),
    [user],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

const withSignedOut: Decorator = (Story) => (
  <SignedOutAuth>
    <Story />
  </SignedOutAuth>
);

/** Where a successful sign-in lands — a probe, since the Dashboard has no mocks. */
function SignedIn() {
  return (
    <Box p={6} textAlign="center">
      <Text fontWeight="semibold">Signed in → /</Text>
      <Text fontSize="sm" color="fg.muted">
        In the app this is the Dashboard.
      </Text>
    </Box>
  );
}

export const routes = (
  <>
    <Route path="/login" element={<Login />} />
    <Route path="/" element={<SignedIn />} />
  </>
);

const branding = (businessType: BussinessType, appTitle = "") =>
  mockRpc(SettingsService, "getBranding", { businessType, appTitle });

const loginOk = mockRpc(AuthService, "login", {
  accessToken: "story-access",
  refreshToken: "story-refresh",
  user: STORY_USERS.owner,
});

/** Everything but `title`, `render` and the device — the story file adds those. */
export const meta = {
  component: Login,
  parameters: {
    router: { initialEntries: ["/login"] },
    msw: mockApi(branding(BussinessType.RETAIL), loginOk),
  },
  decorators: [withSignedOut],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

/** Fill the form and submit, as a user would. */
async function signIn(canvasElement: HTMLElement, email: string, password: string) {
  const canvas = within(canvasElement);
  // selector: "input" — the show/hide toggle's aria-label ("Show password")
  // would otherwise match the password label too.
  await userEvent.type(await canvas.findByLabelText(/^email/i, { selector: "input" }), email);
  await userEvent.type(canvas.getByLabelText(/^(password|kata sandi)/i, { selector: "input" }), password);
  await userEvent.click(canvas.getByRole("button", { name: /sign in|masuk/i }));
}

export const stories = {
  Retail: story("A retail shop with no custom title: the store icon and the built-in brand."),
  Pharmacy: story("Pharmacy mode: the pill icon and the pharmacy brand, read from the public GetBranding.", {
    parameters: { msw: mockApi(branding(BussinessType.PHARMACY_SHOP), loginOk) },
  }),
  CustomTitle: story("The owner set an app title in Settings ▸ General; it replaces the built-in brand.", {
    parameters: { msw: mockApi(branding(BussinessType.RETAIL, "Toko Sumber Rejeki"), loginOk) },
  }),
  ValidationErrors: story(
    "Submitted empty: the Zod errors render under each field, translated through the global " +
      "error map — the Sign-in button is never disabled for validity (app convention).",
    {
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await userEvent.click(await canvas.findByRole("button", { name: /sign in|masuk/i }));
      },
    },
  ),
  WrongPassword: story(
    "Login answers Unauthenticated: the page stays put and raises its invalid-credentials toast.",
    {
      parameters: {
        msw: mockApi(
          branding(BussinessType.RETAIL),
          mockRpcError(AuthService, "login", Code.Unauthenticated, "invalid credentials"),
        ),
      },
      play: async ({ canvasElement }) => {
        await signIn(canvasElement, "owner@toko.test", "salah");
        await expect(await within(document.body).findByRole("status")).toBeVisible();
      },
    },
  ),
  Submitting: story("Login never answers: the Sign-in button holds its spinner.", {
    parameters: {
      msw: mockApi(
        branding(BussinessType.RETAIL),
        mockRpc(AuthService, "login", {}, { delay: "infinite" }),
      ),
    },
    play: async ({ canvasElement }) => signIn(canvasElement, "owner@toko.test", "rahasia"),
  }),
  SignsIn: story("Correct credentials: the session starts and the page redirects to `/`.", {
    play: async ({ canvasElement }) => {
      await signIn(canvasElement, "owner@toko.test", "rahasia");
      await expect(await within(canvasElement).findByText("Signed in → /")).toBeVisible();
    },
  }),
};
