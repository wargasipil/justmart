import { HStack, Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import SearchableSelect from "./SearchableSelect";
import UserAvatar from "./UserAvatar";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

type Product = { id: string; name: string };

const PRODUCTS: Product[] = [
  { id: "p1", name: "Paracetamol 500mg" },
  { id: "p2", name: "Amoxicillin 500mg" },
  { id: "p3", name: "Vitamin C 1000mg" },
  { id: "p4", name: "Ibuprofen 400mg" },
  { id: "p5", name: "Cetirizine 10mg" },
];

// Stands in for a `Search<Domain>` RPC: same async contract, with fake latency
// so the 250ms debounce and the loading state are actually visible.
function fakeSearch(query: string, latency = 250): Promise<Product[]> {
  const needle = query.trim().toLowerCase();
  const rows = needle ? PRODUCTS.filter((p) => p.name.toLowerCase().includes(needle)) : PRODUCTS;
  return new Promise((resolve) => setTimeout(() => resolve(rows), latency));
}

type Person = { id: string; name: string; role: string };
const PEOPLE: Person[] = [
  { id: "u1", name: "Budi Santoso", role: "Kasir" },
  { id: "u2", name: "Siti Rahayu", role: "Admin" },
  { id: "u3", name: "Ahmad Wijaya", role: "Kasir" },
];

function peopleSearch(query: string): Promise<Person[]> {
  const needle = query.trim().toLowerCase();
  const rows = needle ? PEOPLE.filter((p) => p.name.toLowerCase().includes(needle)) : PEOPLE;
  return new Promise((resolve) => setTimeout(() => resolve(rows), 250));
}

const meta = {
  title: "Selects & pickers/SearchableSelect",
  parameters: storyDocs("searchable-select"),
  tags: ["autodocs"],
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * **Async mode — the required one for dynamic data.** Options come from a
 * `Search<Domain>` RPC through `loadOptions`; the wrapper debounces 250ms and
 * guards against a stale response overwriting a newer one. Never preload a
 * whole catalog and filter it client-side.
 */
export const AsyncLoadOptions: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("");
      return (
        <Stack gap={2} maxW="360px">
          <SearchableSelect<Product>
            value={value}
            onChange={setValue}
            loadOptions={fakeSearch}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            placeholder="Cari produk…"
          />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/** Slow backend — the loading text is what the cashier stares at. */
export const AsyncSlow: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("");
      return (
        <Stack gap={2} maxW="360px">
          <SearchableSelect<Product>
            value={value}
            onChange={setValue}
            loadOptions={(q) => fakeSearch(q, 1800)}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            placeholder="Cari produk…"
          />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/** No match — the (already translated) `common.noResults` empty state. */
export const AsyncNoResults: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("");
      return (
        <Stack gap={2} maxW="360px">
          <Text fontSize="xs" color="fg.muted">
            Type anything — this loader always returns nothing.
          </Text>
          <SearchableSelect<Product>
            value={value}
            onChange={setValue}
            loadOptions={() => Promise.resolve([])}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            placeholder="Cari produk…"
          />
        </Stack>
      );
    }
    return <Demo />;
  },
};

/** Sync `items` mode — the carve-out, for ≤20 static options only. */
export const SyncItems: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("");
      return (
        <Stack gap={2} maxW="360px">
          <SearchableSelect<Product>
            value={value}
            onChange={setValue}
            items={PRODUCTS}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            placeholder="Produk"
          />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/**
 * `selectedLabel` — an edit drawer mounts with the id already set, before any
 * search has run. Without it the trigger shows the raw UUID.
 */
export const PreselectedInEditDrawer: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("p3");
      return (
        <Stack gap={2} maxW="360px">
          <SearchableSelect<Product>
            value={value}
            onChange={setValue}
            loadOptions={fakeSearch}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            selectedLabel="Vitamin C 1000mg"
          />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/**
 * `renderItem` — rich rows when one line of text can't tell options apart.
 * It must return exactly ONE element (it's mounted with `asChild`); a fragment
 * or two siblings throws. `version={0}` on the avatar means "no picture", so
 * this story still makes no request.
 */
export const RichRows: Story = {
  render: () => {
    function Demo() {
      const [value, setValue] = useState("");
      return (
        <Stack gap={2} maxW="360px">
          <SearchableSelect<Person>
            value={value}
            onChange={setValue}
            loadOptions={peopleSearch}
            itemToString={(p) => p.name}
            itemToValue={(p) => p.id}
            renderItem={(p) => (
              <HStack gap={3} flex="1" minW={0}>
                <UserAvatar userId={p.id} name={p.name} version={0} />
                <Stack gap={0} flex="1" minW={0}>
                  <Text fontWeight="medium" truncate>
                    {p.name}
                  </Text>
                  <Text fontSize="xs" color="fg.muted" truncate>
                    {p.role}
                  </Text>
                </Stack>
              </HStack>
            )}
            placeholder="Cari pengguna…"
          />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

export const Disabled: Story = {
  render: () => (
    <SearchableSelect<Product>
      value="p1"
      onChange={() => {}}
      loadOptions={fakeSearch}
      itemToString={(p) => p.name}
      itemToValue={(p) => p.id}
      selectedLabel="Paracetamol 500mg"
      disabled
      width="360px"
    />
  ),
};
