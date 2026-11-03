import type { Meta } from "@storybook/react";

import type Settings from "../../../../routes/Settings";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/settings";
import { MOBILE, SCREEN } from "../../../viewports";

// Settings as a whole screen of the mobile version (390px): no rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav.
// The real AppShell wraps the page — the bottom bar and its Menu drawer; a
// phone has no TopBar — so this is the app as a user sees it.
// Below `md` the page's own tabs degrade from a rail to a horizontal strip
// that scrolls sideways (six tabs do not fit 390px), and the panel sits under
// it rather than beside it. That, and every panel's table fitting the width
// without the PAGE scrolling sideways, is what this file is for.
// Data + states live in scenarios/settings.tsx; the shell's own reads are
// added by withShell.

const meta = {
  title: "screens/mobile/pages/settings/Settings",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Settings>;

export default meta;

export const General = withShell(scenario.stories.General);
export const Units = withShell(scenario.stories.Units);
export const UnitsEmpty = withShell(scenario.stories.UnitsEmpty);
export const Printing = withShell(scenario.stories.Printing);
export const PrintingNoConnector = withShell(scenario.stories.PrintingNoConnector);
export const PrintingUsb = withShell(scenario.stories.PrintingUsb);
export const PrintingTcp = withShell(scenario.stories.PrintingTcp);
export const Tunnel = withShell(scenario.stories.Tunnel);
export const TunnelPendingRestart = withShell(scenario.stories.TunnelPendingRestart);
export const TunnelEnvOff = withShell(scenario.stories.TunnelEnvOff);
export const TunnelUnsupported = withShell(scenario.stories.TunnelUnsupported);
export const Backups = withShell(scenario.stories.Backups);
export const BackupsEmpty = withShell(scenario.stories.BackupsEmpty);
export const Updates = withShell(scenario.stories.Updates);
export const UpdatesUpToDate = withShell(scenario.stories.UpdatesUpToDate);
export const UpdatesCheckFailed = withShell(scenario.stories.UpdatesCheckFailed);
export const UpdatesDisabled = withShell(scenario.stories.UpdatesDisabled);
export const Loading = withShell(scenario.stories.Loading);
