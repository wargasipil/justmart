import type { Meta } from "@storybook/react";
import { Routes } from "react-router-dom";

import type Login from "../../../../routes/Login";
import * as scenario from "../../../scenarios/login";
import { MOBILE, MOBILE_PAGE } from "../../../viewports";

// The sign-in page on the mobile version (390px). Login is rendered bare in
// the app — no AppShell, no bottom bar — so this is the page on its own.
// Data + states live in scenarios/login.tsx.

const meta = {
  title: "screens/mobile/pages/users/Login",
  ...scenario.meta,
  render: () => <Routes>{scenario.routes}</Routes>,
  globals: MOBILE,
  parameters: { ...scenario.meta.parameters, ...MOBILE_PAGE },
} satisfies Meta<typeof Login>;

export default meta;

export const Retail = scenario.stories.Retail;
export const Pharmacy = scenario.stories.Pharmacy;
export const CustomTitle = scenario.stories.CustomTitle;
export const ValidationErrors = scenario.stories.ValidationErrors;
export const WrongPassword = scenario.stories.WrongPassword;
export const Submitting = scenario.stories.Submitting;
export const SignsIn = scenario.stories.SignsIn;
