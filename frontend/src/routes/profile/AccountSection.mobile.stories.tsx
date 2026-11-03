import type { Meta } from "@storybook/react";

import { MOBILE, MOBILE_PAGE } from "../../screens/viewports";
import AccountSection from "./AccountSection";
import * as scenario from "./AccountSection.scenario";

// The Akun card of /profile at phone width (390px), with the same gutter
// AppShell's <main> gives it: avatar on top, identity underneath and wrapping.
// States live in AccountSection.scenario.tsx, shared with the desktop file.

const meta = {
  title: "components/users/AccountMobile",
  ...scenario.meta,
  component: AccountSection,
  globals: MOBILE,
  parameters: { ...scenario.meta.parameters, ...MOBILE_PAGE },
} satisfies Meta<typeof AccountSection>;

export default meta;

export const NoPicture = scenario.stories.NoPicture;
export const WithPicture = scenario.stories.WithPicture;
export const Cashier = scenario.stories.Cashier;
export const LongName = scenario.stories.LongName;
