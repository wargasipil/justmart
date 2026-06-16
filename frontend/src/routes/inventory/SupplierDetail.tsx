import { useEffect, useMemo, useState } from "react";
import { Badge, Box, Heading, HStack, Input, SimpleGrid, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import { formatDiscount, formatMoney, formatUnixOrDash } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useProductRefs } from "../../queries/refs";
import { useSupplierQuery, useSupplierRestocksQuery } from "../../queries/suppliers";

export default function SupplierDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const supQ = useSupplierQuery(id);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  const { page, setPage, pageSize, setPageSize } = usePageState(`${id}|${query}`);
  const restocksQ = useSupplierRestocksQuery(id, { query, page, pageSize, enabled: !!id });
  const productRefs = useProductRefs(
    useMemo(() => restocksQ.rows.map((r) => r.productId), [restocksQ.rows]),
  );

  if (supQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const sup = supQ.data;
  if (!sup) {
    return (
      <Box p={8}>
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  return (
    <Box>
      <BackButton to="/inventory/suppliers" />
      <PageHeader
        breadcrumbs={[{ label: t("inventory.suppliers.title"), to: "/inventory/suppliers" }, { label: sup.name }]}
        title={sup.name}
      />

      <Stack gap={6}>
        <Box>
          <Heading size="sm" mb={3}>
            {t("inventory.suppliers.infoSection")}
          </Heading>
          <SimpleGrid columns={{ base: 2, md: 4 }} gap={3}>
            <Field label={t("inventory.suppliers.code")} value={sup.code} mono />
            <Field label={t("inventory.suppliers.email")} value={sup.contactEmail || "—"} />
            <Field label={t("inventory.suppliers.phone")} value={sup.phone || "—"} />
            <Field label={t("inventory.suppliers.address")} value={sup.address || "—"} />
            <Field label={t("inventory.suppliers.bankName")} value={sup.bankName || "—"} />
            <Field label={t("inventory.suppliers.bankAccountNumber")} value={sup.bankAccountNumber || "—"} mono />
            <Field label={t("inventory.suppliers.bankAccountHolder")} value={sup.bankAccountHolder || "—"} />
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("common.active")}
              </Text>
              <Badge colorPalette={sup.active ? "green" : "gray"}>
                {sup.active ? t("common.active") : t("common.inactive")}
              </Badge>
            </Box>
          </SimpleGrid>
        </Box>

        <Box>
          <HStack justify="space-between" mb={3} wrap="wrap" gap={2}>
            <Heading size="sm">{t("inventory.suppliers.restockSection")}</Heading>
            <Box position="relative">
              <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
                <Search size={14} />
              </Box>
              <Input
                size="sm"
                pl={7}
                width="240px"
                placeholder={t("inventory.suppliers.restockSearchPlaceholder")}
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
              />
            </Box>
          </HStack>
          <Box overflowX="auto">
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("inventory.suppliers.restockProduct")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockPrice")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockQty")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockDiscount")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.products.lastRestockCreated")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.products.lastRestockArrived")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {restocksQ.rows.map((r) => (
                  <Table.Row key={r.productId}>
                    <Table.Cell>{productRefs.get(r.productId)?.name ?? "—"}</Table.Cell>
                    <Table.Cell textAlign="end">{formatMoney(r.lastPrice)}</Table.Cell>
                    <Table.Cell textAlign="end">{r.lastQty.toString()}</Table.Cell>
                    <Table.Cell textAlign="end">{formatDiscount(r.lastDiscountType, r.lastDiscountValue)}</Table.Cell>
                    <Table.Cell>{formatUnixOrDash(r.lastCreatedAt)}</Table.Cell>
                    <Table.Cell>{formatUnixOrDash(r.lastArrivedAt)}</Table.Cell>
                  </Table.Row>
                ))}
                {restocksQ.rows.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={6}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {t("inventory.suppliers.restockEmpty")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </Box>
          <Pagination
            page={page}
            pageSize={pageSize}
            total={restocksQ.total}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
          />
        </Box>
      </Stack>
    </Box>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}
