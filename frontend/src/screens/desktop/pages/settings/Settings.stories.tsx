import type { Meta } from "@storybook/react";

import type Settings from "../../../../routes/Settings";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/settings";
import { DESKTOP, SCREEN } from "../../../viewports";

// Settings as a whole screen of the desktop version (1440px): the fixed sidebar rail beside the page.
// The real AppShell wraps the page — sidebar, TopBar with the warehouse
// picker, low-stock bell and user menu — so this is the app as a user sees it.
// At this width the page's own tab strip is a VERTICAL rail beside the panel
// (useTabsOrientation), which is the layout the mobile file cannot show.
// Data + states live in scenarios/settings.tsx; the shell's own reads are
// added by withShell.

const meta = {
  title: "screens/desktop/pages/settings/Settings",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
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
