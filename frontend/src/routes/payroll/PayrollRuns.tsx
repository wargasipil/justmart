import { useState } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import EnumSelect from "../../components/EnumSelect";
import Pagination from "../../components/Pagination";
import { usePayrollRunsQuery } from "../../queries/payroll";
import { usePageState } from "../../lib/pagination";
import { formatMoney, formatUnix } from "../../lib/format";
import { RUN_STATUSES, RunStatusBadge, useMonthLabel } from "./shared";

export default function PayrollRuns() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const monthLabel = useMonthLabel();
  const [status, setStatus] = useState("");
  const [yearInput, setYearInput] = useState("");

  const year = /^\d{4}$/.test(yearInput) ? Number(yearInput) : 0;
  const { page, setPage, pageSize, setPageSize } = usePageState(`${status}|${year}`);
  const q = usePayrollRunsQuery({ status, year, page, pageSize });

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2}>
          <Box minW="160px">
            <EnumSelect
              size="sm"
              value={status}
              onChange={setStatus}
              items={["", ...RUN_STATUSES]}
              itemToString={(s) => (s === "" ? t("payroll.filterAllStatus") : t(`payroll.runStatus.${s.toLowerCase()}`))}
              itemToValue={(s) => s}
              placeholder={t("payroll.filterAllStatus")}
            />
          </Box>
          <Input
            size="sm"
            width="120px"
            placeholder={t("payroll.filterYear")}
            value={yearInput}
            onChange={(e) => setYearInput(e.target.value)}
          />
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => navigate("/payroll/runs/new")}>
          <Plus size={16} />
          {t("payroll.run.addTitle")}
        </Button>
      </HStack>

      {q.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("payroll.run.runNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("payroll.run.period")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.status")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("payroll.run.gross")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("payroll.run.deductions")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("payroll.run.net")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.created")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {q.rows.map((r) => (
              <Table.Row key={r.id} cursor="pointer" _hover={{ bg: "bg.muted" }} onClick={() => navigate(`/payroll/runs/${r.id}`)}>
                <Table.Cell fontFamily="mono">{r.runNo}</Table.Cell>
                <Table.Cell>{`${monthLabel(r.periodMonth)} ${r.periodYear}`}</Table.Cell>
                <Table.Cell>
                  <RunStatusBadge status={r.status} />
                </Table.Cell>
                <Table.Cell textAlign="end">{formatMoney(Number(r.totalGross))}</Table.Cell>
                <Table.Cell textAlign="end">{formatMoney(Number(r.totalDeductions))}</Table.Cell>
                <Table.Cell textAlign="end">{formatMoney(Number(r.totalNet))}</Table.Cell>
                <Table.Cell>{formatUnix(Number(r.createdAt))}</Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={7}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Pagination page={page} pageSize={pageSize} total={q.total} onPageChange={setPage} onPageSizeChange={setPageSize} />
    </Stack>
  );
}
