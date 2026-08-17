import { Button, HStack, Stack, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import { toast } from "./toaster";
import { storyDocs } from "../routes/dev/storyDocs";

// The toaster is a module-level singleton mounted once by <AppToaster/> — the
// preview decorator mounts it, exactly as main.tsx does.

const meta = {
  title: "Feedback/toast",
  parameters: storyDocs("toast"),
  tags: ["autodocs"],
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  render: () => (
    <HStack gap={2} flexWrap="wrap">
      <Button size="sm" variant="outline" onClick={() => toast.success("Tersimpan", "Pemasok dibuat")}>
        success
      </Button>
      <Button size="sm" variant="outline" onClick={() => toast.info("Perhatian", "Draf dipulihkan")}>
        info
      </Button>
      <Button size="sm" variant="outline" onClick={() => toast.error("Gagal", "Stok tidak cukup")}>
        error
      </Button>
    </HStack>
  ),
};

/**
 * `toast.fromError` translates the backend's stable tokens. A KNOWN token
 * (`product.sku_taken`) becomes real copy; an UNKNOWN token-shaped or raw-DB
 * string is genericized to `errors.generic`, which is what stops raw SQL text
 * ever reaching a shop owner. Flip the toolbar globe to see both translate.
 */
export const FromServerError: Story = {
  render: () => (
    <Stack gap={3} align="flex-start">
      <HStack gap={2} flexWrap="wrap">
        <Button
          size="sm"
          variant="outline"
          onClick={() => toast.fromError(new Error("product.sku_taken"))}
        >
          known token
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => toast.fromError(new Error("some.unmapped_token"))}
        >
          unknown token
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() =>
            toast.fromError(
              new Error('ERROR: duplicate key value violates unique constraint "products_sku_idx"'),
            )
          }
        >
          raw DB string
        </Button>
      </HStack>
      <Text fontSize="xs" color="fg.muted" maxW="480px">
        A field-attributable error belongs ON the field via useServerFormErrors, not in a toast —
        these are the fallbacks for everything else.
      </Text>
    </Stack>
  ),
};

/** Several at once — they stack rather than replacing each other. */
export const Stacked: Story = {
  render: () => (
    <Button
      size="sm"
      colorPalette="blue"
      onClick={() => {
        toast.success("Tersimpan", "Pemasok dibuat");
        toast.info("Perhatian", "Draf dipulihkan");
        toast.error("Gagal", "Stok tidak cukup");
      }}
    >
      Munculkan tiga
    </Button>
  ),
};
