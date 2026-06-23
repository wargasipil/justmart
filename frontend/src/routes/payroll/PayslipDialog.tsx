import { Box, Button, Dialog, Heading, HStack, IconButton, Portal, Separator, Stack, Text } from "@chakra-ui/react";
import { Printer, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Payslip, PayrollRun } from "../../gen/payroll_iface/v1/payroll_pb";
import { usePrintPayslipMutation } from "../../queries/payroll";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useMonthLabel } from "./shared";

export default function PayslipDialog({
  open,
  onClose,
  payslip,
  run,
}: {
  open: boolean;
  onClose: () => void;
  payslip: Payslip | null;
  run: PayrollRun | null;
}) {
  const { t } = useTranslation();
  const monthLabel = useMonthLabel();
  const print = usePrintPayslipMutation();

  const earnings = (payslip?.components ?? []).filter((c) => c.kind === "EARNING");
  const deductions = (payslip?.components ?? []).filter((c) => c.kind === "DEDUCTION");

  const onPrint = () => {
    if (!payslip) return;
    print.mutate(
      { payslipId: payslip.id },
      { onSuccess: () => toast.success(t("payroll.payslip.printed")) },
    );
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => { if (!d.open) onClose(); }} size="md">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between" w="full">
                <Heading size="md">{t("payroll.payslip.title")}</Heading>
                <IconButton aria-label={t("common.close")} variant="ghost" size="sm" onClick={onClose}>
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            <Dialog.Body>
              {payslip && run && (
                <Stack gap={3}>
                  <Box>
                    <Text fontWeight="semibold">{payslip.employeeName}</Text>
                    <Text fontSize="sm" color="fg.muted">
                      {payslip.employeeCode}
                      {payslip.position ? ` · ${payslip.position}` : ""}
                    </Text>
                    <Text fontSize="sm" color="fg.muted">
                      {`${monthLabel(run.periodMonth)} ${run.periodYear} · ${run.runNo}`}
                    </Text>
                  </Box>
                  <Separator />
                  <Box>
                    <Text fontSize="sm" fontWeight="medium" mb={1}>
                      {t("payroll.payslip.earnings")}
                    </Text>
                    {earnings.map((c) => (
                      <AmountRow key={c.id} label={c.label} amount={Number(c.amount)} />
                    ))}
                    <AmountRow label={t("payroll.payslip.totalEarnings")} amount={Number(payslip.gross)} bold />
                  </Box>
                  <Box>
                    <Text fontSize="sm" fontWeight="medium" mb={1}>
                      {t("payroll.payslip.deductions")}
                    </Text>
                    {deductions.length === 0 && (
                      <Text fontSize="sm" color="fg.muted">
                        —
                      </Text>
                    )}
                    {deductions.map((c) => (
                      <AmountRow key={c.id} label={c.label} amount={Number(c.amount)} />
                    ))}
                    <AmountRow label={t("payroll.payslip.totalDeductions")} amount={Number(payslip.totalDeductions)} bold />
                  </Box>
                  <Separator />
                  <AmountRow label={t("payroll.payslip.net")} amount={Number(payslip.net)} bold />
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer borderTopWidth="1px">
              <HStack justify="flex-end" w="full" gap={2}>
                <Button variant="ghost" onClick={onClose}>
                  {t("common.close")}
                </Button>
                <Button colorPalette="blue" loading={print.isPending} onClick={onPrint}>
                  <Printer size={16} />
                  {t("payroll.payslip.print")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function AmountRow({ label, amount, bold }: { label: string; amount: number; bold?: boolean }) {
  return (
    <HStack justify="space-between" py={0.5}>
      <Text fontSize="sm" fontWeight={bold ? "semibold" : "normal"}>
        {label}
      </Text>
      <Text fontSize="sm" fontWeight={bold ? "semibold" : "normal"}>
        {formatMoney(amount)}
      </Text>
    </HStack>
  );
}
