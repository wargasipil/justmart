import { Badge } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

// Indonesian-friendly month options (1..12) for the run period picker.
export const MONTH_VALUES = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12] as const;

export const RUN_STATUSES = ["DRAFT", "APPROVED", "PAID", "VOIDED"] as const;

const STATUS_PALETTE: Record<string, string> = {
  DRAFT: "gray",
  APPROVED: "blue",
  PAID: "green",
  VOIDED: "red",
};

export function RunStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return (
    <Badge colorPalette={STATUS_PALETTE[status] ?? "gray"}>
      {t(`payroll.runStatus.${status.toLowerCase()}`)}
    </Badge>
  );
}

// monthLabel maps a 1..12 month number to its localized name via i18n.
export function useMonthLabel() {
  const { t } = useTranslation();
  return (m: number) => t(`payroll.months.${m}`);
}
