import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import Pagination from "./Pagination";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Data display/Pagination",
  component: Pagination,
  parameters: storyDocs("pagination"),
  tags: ["autodocs"],
  argTypes: { onPageChange: { control: false }, onPageSizeChange: { control: false } },
  // onPageChange is replaced by the story's own state setter in `render`.
  args: { page: 0, pageSize: 25, total: 137, onPageChange: () => {} },
  render: (args) => {
    function Demo() {
      const [page, setPage] = useState(args.page);
      const [pageSize, setPageSize] = useState(args.pageSize);
      return (
        <Pagination
          {...args}
          page={page}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={args.onPageSizeChange === undefined ? undefined : setPageSize}
        />
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof Pagination>;

export default meta;
type Story = StoryObj<typeof meta>;

/** `page` is 0-based; `total` is the count the List RPC reports for the SAME filters. */
export const Default: Story = {
  args: { onPageSizeChange: () => {} },
};

export const MiddlePage: Story = {
  args: { page: 2, onPageSizeChange: () => {} },
};

export const LastPage: Story = {
  args: { page: 5, onPageSizeChange: () => {} },
};

/** One page of results — both arrows are dead ends. */
export const SinglePage: Story = {
  args: { total: 12, onPageSizeChange: () => {} },
};

/**
 * Zero rows. Still rendered, so the user sees "0 of 0" rather than a table that
 * just ends with no explanation.
 */
export const Empty: Story = {
  args: { total: 0, onPageSizeChange: () => {} },
};

/** Omit `onPageSizeChange` to hide the size picker. */
export const WithoutSizePicker: Story = {
  args: { total: 137 },
};
