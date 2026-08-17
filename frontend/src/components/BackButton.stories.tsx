import type { Meta, StoryObj } from "@storybook/react";

import BackButton from "./BackButton";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Layout & page chrome/BackButton",
  component: BackButton,
  parameters: storyDocs("back-button"),
  tags: ["autodocs"],
} satisfies Meta<typeof BackButton>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Points at the parent list. This is the form every detail page should use. */
export const ToParentList: Story = {
  args: { to: "/products" },
};

/**
 * No `to` — falls back to `navigate(-1)`. Only right when the parent genuinely
 * depends on where the user came from; prefer an explicit `to` otherwise, so
 * a deep-linked page still has somewhere to go.
 */
export const HistoryFallback: Story = {};

export const CustomLabel: Story = {
  args: { to: "/purchasing", label: "Kembali ke restock" },
};
