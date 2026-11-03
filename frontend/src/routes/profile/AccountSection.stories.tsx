import { Box } from "@chakra-ui/react";
import type { Decorator, Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import AccountSection from "./AccountSection";
import * as scenario from "./AccountSection.scenario";

// The Akun card of /profile, desktop layout: avatar beside the identity.
// States live in AccountSection.scenario.tsx, shared with the mobile file.

// On desktop the card sits in the tab panel beside the rail (the Tabs root is
// maxW 4xl, minus the 180px rail), so it's framed at that width rather than
// stretched across a 1440px canvas.
const panelWidth: Decorator = (Story) => (
  <Box maxW="700px">
    <Story />
  </Box>
);

const meta = {
  title: "components/users/Account",
  ...scenario.meta,
  component: AccountSection,
  decorators: [...scenario.meta.decorators, panelWidth],
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof AccountSection>;

export default meta;

export const NoPicture = scenario.stories.NoPicture;
export const WithPicture = scenario.stories.WithPicture;
export const Cashier = scenario.stories.Cashier;
export const LongName = scenario.stories.LongName;
