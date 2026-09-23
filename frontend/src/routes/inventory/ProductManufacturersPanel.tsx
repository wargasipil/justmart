import { Badge, Box, Button, HStack, IconButton, Stack, Text } from "@chakra-ui/react";
import { Star, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import ManufacturerSelect, { manufacturerLabel } from "../../components/ManufacturerSelect";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { useSetProductManufacturersMutation } from "../../queries/products";
import { useManufacturerRefs } from "../../queries/refs";

/**
 * A product's APPROVED SOURCES: every pabrik it may be bought from.
 *
 * The list is intent, not history — which factory a given delivery actually
 * came from is recorded on its batch, and nothing here can change that. That
 * separation is the whole point of the model: a shop buying one generic from
 * three pabrik used to have a single column that the newest restock overwrote,
 * silently relabelling every lot already on the shelf.
 *
 * One of the sources is the USUAL one (the primary). It is what the restock
 * form suggests on a new line and what other surfaces print when they have room
 * for one name; it is always a member of the list, which the server enforces.
 *
 * Every edit sends the WHOLE set, like the unit editor: a removal is expressed
 * by absence. That keeps "add", "remove" and "make usual" as one round trip each
 * with no partial states to reconcile.
 */
export default function ProductManufacturersPanel({ product }: { product: Product }) {
  const { t } = useTranslation();
  const setMut = useSetProductManufacturersMutation();
  const [adding, setAdding] = useState("");

  const ids = product.manufacturerIds;
  const refs = useManufacturerRefs(useMemo(() => ids, [ids]));

  // No field to hang an error on -- this panel is chips and buttons, not a
  // form -- so refusals ride the global error->toast rule, which already
  // translates a stable token like manufacturer.primary_not_listed.
  const save = (next: string[], primary: string) =>
    setMut.mutate({ productId: product.id, manufacturerIds: next, primaryManufacturerId: primary });

  const add = (id: string) => {
    setAdding("");
    if (!id || ids.includes(id)) return;
    // A first source becomes the usual one; later ones never displace it.
    save([...ids, id], product.manufacturerId || id);
  };
  const remove = (id: string) => {
    const next = ids.filter((x) => x !== id);
    // Dropping the usual one hands the title to whatever is left, or clears it
    // when nothing is: the server would do this anyway, but sending "" rather
    // than a stale id keeps the request honest about what we are asking for.
    save(next, product.manufacturerId === id ? "" : product.manufacturerId);
  };

  return (
    <Stack gap={3} p={4}>
      <Text fontSize="xs" color="fg.muted">
        {t("inventory.products.manufacturersDescription")}
      </Text>

      {ids.length === 0 && (
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.noManufacturers")}
        </Text>
      )}

      <Stack gap={2}>
        {ids.map((id) => (
          <HStack key={id} justify="space-between" gap={2}>
            <HStack gap={2} minW={0}>
              <Text fontSize="sm" truncate>
                {manufacturerLabel(refs.get(id)) ?? "—"}
              </Text>
              {id === product.manufacturerId && (
                <Badge size="sm" colorPalette="blue">
                  {t("inventory.products.primaryManufacturer")}
                </Badge>
              )}
            </HStack>
            <HStack gap={1}>
              {id !== product.manufacturerId && (
                <IconButton
                  aria-label={t("inventory.products.makePrimaryManufacturer")}
                  size="2xs"
                  variant="ghost"
                  onClick={() => save(ids, id)}
                >
                  <Star size={12} />
                </IconButton>
              )}
              <IconButton
                aria-label={t("common.delete")}
                size="2xs"
                variant="ghost"
                colorPalette="red"
                onClick={() => remove(id)}
              >
                <Trash2 size={12} />
              </IconButton>
            </HStack>
          </HStack>
        ))}
      </Stack>

      <Box>
        <ManufacturerSelect
          size="sm"
          value={adding}
          onChange={add}
          placeholder={t("inventory.products.addManufacturer")}
        />
      </Box>
      {setMut.isPending && (
        <Button size="xs" variant="ghost" loading disabled alignSelf="flex-start">
          {t("common.save")}
        </Button>
      )}
    </Stack>
  );
}
