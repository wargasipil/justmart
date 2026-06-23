import { useState } from "react";
import { Box, Button, Grid, HStack, Input, Stack, Text, Textarea } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import BackButton from "../../components/BackButton";
import EnumSelect from "../../components/EnumSelect";
import PageHeader from "../../components/PageHeader";
import { useCreatePayrollRunMutation } from "../../queries/payroll";
import { toast } from "../../lib/toaster";
import { MONTH_VALUES, useMonthLabel } from "./shared";

export default function NewPayrollRun() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const monthLabel = useMonthLabel();
  const now = new Date();
  const [year, setYear] = useState<number>(now.getFullYear());
  const [month, setMonth] = useState<number>(now.getMonth() + 1);
  const [note, setNote] = useState("");
  const create = useCreatePayrollRunMutation();

  const submit = async () => {
    try {
      const res = await create.mutateAsync({ periodYear: year, periodMonth: month, note });
      toast.success(t("payroll.run.generated"));
      navigate(`/payroll/runs/${res.run!.id}`);
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <BackButton to="/payroll/runs" />
      <PageHeader
        breadcrumbs={[
          { label: t("nav.payroll"), to: "/payroll/runs" },
          { label: t("payroll.run.addTitle") },
        ]}
        title={t("payroll.run.addTitle")}
        description={t("payroll.run.addDescription")}
      />

      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5} maxW="lg">
        <Stack gap={4}>
          <Grid templateColumns="repeat(2, 1fr)" gap={3}>
            <Box>
              <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
                {t("payroll.run.month")}
              </Text>
              <EnumSelect
                value={String(month)}
                onChange={(v) => setMonth(Number(v))}
                items={MONTH_VALUES.map((m) => String(m))}
                itemToString={(s) => monthLabel(Number(s))}
                itemToValue={(s) => s}
              />
            </Box>
            <Box>
              <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
                {t("payroll.run.year")}
              </Text>
              <Input type="number" value={year} onChange={(e) => setYear(Number(e.target.value) || 0)} />
            </Box>
          </Grid>
          <Box>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("payroll.run.note")}
            </Text>
            <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} />
          </Box>
          <HStack justify="flex-end" gap={2}>
            <Button variant="ghost" onClick={() => navigate("/payroll/runs")}>
              {t("common.cancel")}
            </Button>
            <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
              {t("payroll.run.generate")}
            </Button>
          </HStack>
        </Stack>
      </Box>
    </Box>
  );
}
