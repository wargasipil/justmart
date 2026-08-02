import {
  Badge,
  Box,
  Button,
  Card,
  Grid,
  HStack,
  IconButton,
  Spinner,
  Stack,
  Text,
} from "@chakra-ui/react";
import { ArchiveRestore, Archive, Pencil, Plus, Users } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ConfirmDialog from "../../components/ConfirmDialog";
import EnumSelect from "../../components/EnumSelect";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import { Role } from "../../gen/auth_iface/v1/policy_pb";
import type { DiningTable } from "../../gen/table_iface/v1/table_pb";
import { useAuth } from "../../lib/auth";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import {
  areasOf,
  useArchiveTableMutation,
  useOpenTableMutation,
  useTablesQuery,
  useUnarchiveTableMutation,
} from "../../queries/tables";
import OpenTableDialog from "./OpenTableDialog";
import TableDrawer from "./tableDrawers";

// The floor plan. Tiles rather than a table row per table: a waiter reads the
// room at a glance and taps, and the occupied/free split has to be visible
// without parsing text.
export default function Tables() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { user } = useAuth();
  const isManager = user?.role === Role.OWNER || user?.role === Role.PHARMACIST;

  const [area, setArea] = useState("");
  const [showArchived, setShowArchived] = useState(false);
  const { page, setPage, pageSize, setPageSize } = usePageState(`${area}|${showArchived}`);
  const tablesQ = useTablesQuery({
    area,
    includeInactive: showArchived,
    page,
    pageSize,
  });

  const [drawerFor, setDrawerFor] = useState<DiningTable | null | undefined>(undefined);
  const [opening, setOpening] = useState<DiningTable | null>(null);
  const [archiving, setArchiving] = useState<DiningTable | null>(null);
  const archive = useArchiveTableMutation();
  const unarchive = useUnarchiveTableMutation();
  const openTable = useOpenTableMutation();

  // Tapping an OCCUPIED table resumes its bill; tapping a free one asks for the
  // cover count first. Both land in POS on the same sale.
  const onTileClick = async (tbl: DiningTable) => {
    if (!tbl.active) return;
    if (tbl.openSaleId) {
      navigate(`/pos?sale=${tbl.openSaleId}`);
      return;
    }
    setOpening(tbl);
  };

  const onConfirmOpen = async (guestCount: number) => {
    if (!opening) return;
    try {
      const res = await openTable.mutateAsync({ tableId: opening.id, guestCount });
      setOpening(null);
      navigate(`/pos?sale=${res.saleId}`);
    } catch {
      /* toast handled by the form/global handler */
    }
  };

  const onConfirmArchive = async () => {
    if (!archiving) return;
    try {
      if (archiving.active) {
        await archive.mutateAsync(archiving.id);
        toast.success(t("tables.archived"));
      } else {
        await unarchive.mutateAsync(archiving.id);
        toast.success(t("tables.unarchived"));
      }
      setArchiving(null);
    } catch {
      /* toast handled globally */
    }
  };

  const areaOptions = [
    { value: "", label: t("tables.allAreas") },
    ...areasOf(tablesQ.rows).map((a) => ({ value: a, label: a })),
  ];

  return (
    <Box>
      <PageHeader
        title={t("tables.title")}
        description={t("tables.description")}
        titleBadge={
          tablesQ.total > 0 ? (
            <Badge colorPalette={tablesQ.occupied > 0 ? "orange" : "green"}>
              {t("tables.occupancy", { occupied: tablesQ.occupied, total: tablesQ.total })}
            </Badge>
          ) : undefined
        }
        actions={
          isManager ? (
            <Button colorPalette="blue" onClick={() => setDrawerFor(null)}>
              <Plus size={16} />
              {t("common.add")}
            </Button>
          ) : undefined
        }
      />

      <HStack gap={3} mb={4} wrap="wrap">
        <Box minW="200px">
          <EnumSelect
            value={area}
            onChange={setArea}
            items={areaOptions}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
            size="sm"
            placeholder={t("tables.allAreas")}
          />
        </Box>
        {isManager && (
          <Button size="sm" variant="ghost" onClick={() => setShowArchived((v) => !v)}>
            {showArchived ? t("tables.hideArchived") : t("tables.showArchived")}
          </Button>
        )}
      </HStack>

      {tablesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : tablesQ.rows.length === 0 ? (
        <Box p={8} textAlign="center" color="fg.muted">
          <Text>{t("tables.empty")}</Text>
        </Box>
      ) : (
        <Grid templateColumns="repeat(auto-fill, minmax(180px, 1fr))" gap={3}>
          {tablesQ.rows.map((tbl) => {
            const occupied = !!tbl.openSaleId;
            return (
              <Card.Root
                key={tbl.id}
                borderWidth="1px"
                borderColor={occupied ? "orange.solid" : "border"}
                bg={tbl.active ? "bg.subtle" : "bg.muted"}
                opacity={tbl.active ? 1 : 0.6}
                cursor={tbl.active ? "pointer" : "default"}
                onClick={() => onTileClick(tbl)}
                _hover={tbl.active ? { borderColor: "colorPalette.solid" } : undefined}
                colorPalette="blue"
              >
                <Card.Body p={4}>
                  <Stack gap={2}>
                    <HStack justify="space-between" align="start">
                      <Text fontSize="2xl" fontWeight="bold" lineHeight="1">
                        {tbl.code}
                      </Text>
                      <Badge colorPalette={occupied ? "orange" : "green"} size="sm">
                        {occupied ? t("tables.occupied") : t("tables.free")}
                      </Badge>
                    </HStack>

                    {tbl.name && (
                      <Text fontSize="sm" color="fg.muted" truncate title={tbl.name}>
                        {tbl.name}
                      </Text>
                    )}

                    <HStack gap={3} fontSize="xs" color="fg.muted">
                      {tbl.area && <Text>{tbl.area}</Text>}
                      {tbl.seats > 0 && (
                        <HStack gap={1}>
                          <Users size={12} />
                          <Text>{tbl.seats}</Text>
                        </HStack>
                      )}
                    </HStack>

                    {occupied && (
                      <Stack gap={0} pt={1} borderTopWidth="1px">
                        <Text fontSize="sm" fontWeight="medium">
                          {formatMoney(tbl.openTotal)}
                        </Text>
                        <Text fontSize="xs" color="fg.muted">
                          {t("tables.openItems", { count: tbl.openItemCount })}
                          {tbl.guestCount > 0 &&
                            ` · ${t("tables.guests", { count: tbl.guestCount })}`}
                        </Text>
                      </Stack>
                    )}

                    {isManager && (
                      <HStack gap={1} pt={1} onClick={(e) => e.stopPropagation()}>
                        <IconButton
                          aria-label={t("common.edit")}
                          size="xs"
                          variant="ghost"
                          onClick={() => setDrawerFor(tbl)}
                        >
                          <Pencil size={14} />
                        </IconButton>
                        <IconButton
                          aria-label={tbl.active ? t("common.archive") : t("common.unarchive")}
                          size="xs"
                          variant="ghost"
                          onClick={() => setArchiving(tbl)}
                        >
                          {tbl.active ? <Archive size={14} /> : <ArchiveRestore size={14} />}
                        </IconButton>
                      </HStack>
                    )}
                  </Stack>
                </Card.Body>
              </Card.Root>
            );
          })}
        </Grid>
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={tablesQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <TableDrawer
        open={drawerFor !== undefined}
        table={drawerFor}
        onClose={() => setDrawerFor(undefined)}
      />

      <OpenTableDialog
        table={opening}
        open={opening != null}
        isPending={openTable.isPending}
        onConfirm={onConfirmOpen}
        onCancel={() => setOpening(null)}
      />

      <ConfirmDialog
        open={archiving != null}
        title={archiving?.active ? t("tables.archiveTitle") : t("tables.unarchiveTitle")}
        body={
          archiving?.active
            ? t("tables.archiveConfirm", { code: archiving?.code ?? "" })
            : t("tables.unarchiveConfirm", { code: archiving?.code ?? "" })
        }
        confirmLabel={archiving?.active ? t("common.archive") : t("common.unarchive")}
        cancelLabel={t("common.cancel")}
        confirmColorPalette={archiving?.active ? "red" : "blue"}
        loading={archive.isPending || unarchive.isPending}
        onConfirm={onConfirmArchive}
        onCancel={() => setArchiving(null)}
      />
    </Box>
  );
}
