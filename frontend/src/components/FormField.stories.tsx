import { Button, HStack, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import type { Meta, StoryObj } from "@storybook/react";

import FormField from "./FormField";
import { storyDocs } from "../routes/dev/storyDocs";
import { toast } from "../lib/toaster";

// FormField is generic over the form shape and takes RHF's `control` object, so
// there is nothing meaningful to expose as a Storybook arg — each story mounts a
// real useForm instead. The props table in the docs tab comes from the gallery
// registry (see storyDocs).

const Schema = z.object({
  name: z.string().min(1),
  password: z.string().min(8),
  // money / number fields emit a RAW DIGIT STRING, so a string field plus a
  // z.coerce.* at the call site is the shape real forms use.
  price: z.string(),
  qty: z.string(),
  expiry: z.string(),
  sku: z.string(),
});
type Values = z.infer<typeof Schema>;

const EMPTY: Values = { name: "", password: "", price: "", qty: "", expiry: "", sku: "SKU-0001" };

function DemoForm({ prefill }: { prefill?: Partial<Values> }) {
  const form = useForm<Values>({
    resolver: zodResolver(Schema),
    defaultValues: { ...EMPTY, ...prefill },
  });
  return (
    <Stack gap={4} maxW="480px">
      <FormField
        control={form.control}
        name="name"
        label="Nama"
        required
        placeholder="Paracetamol 500mg"
      />
      <FormField
        control={form.control}
        name="password"
        label="Kata sandi"
        type="password"
        passwordToggle
        helperText="min 8 characters"
      />
      <FormField control={form.control} name="price" label="Harga satuan" money />
      <FormField control={form.control} name="qty" label="Jumlah" number />
      <FormField control={form.control} name="expiry" label="Kedaluwarsa" type="date" />
      <FormField
        control={form.control}
        name="sku"
        label="SKU"
        disabled
        helperText="immutable on edit"
      />
      <HStack>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={form.handleSubmit((v) => toast.success("Submit", JSON.stringify(v)))}
        >
          Simpan
        </Button>
        <Button size="sm" variant="ghost" onClick={() => form.reset()}>
          Reset
        </Button>
      </HStack>
    </Stack>
  );
}

const meta = {
  title: "Forms & inputs/FormField",
  parameters: storyDocs("form-field"),
  tags: ["autodocs"],
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Every input variant in one form. Note the Save button is NOT gated on
 * validity — this codebase has no `formState.isValid` and no `mode:
 * "onChange"`. Submitting runs the schema and the message lands under the
 * offending field.
 */
export const AllVariants: Story = {
  render: () => <DemoForm />,
};

/**
 * Press **Simpan** on the empty form: the messages are the app's translated
 * `validation.*` copy, not Zod's English defaults — the schemas above declare
 * plain rules (`.min(1)`, `.min(8)`) with no inline message, and the global
 * error map translates them. Flip the toolbar globe to see them switch.
 */
export const ValidationErrors: Story = {
  render: () => <DemoForm />,
};

/** Edit mode: fields prefilled, the immutable one disabled but still in the schema. */
export const Prefilled: Story = {
  render: () => (
    <DemoForm
      prefill={{ name: "Paracetamol 500mg", price: "2500", qty: "100", expiry: "2027-04-30" }}
    />
  ),
};
