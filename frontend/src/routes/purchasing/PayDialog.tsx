import {
  Box,
  Button,
  Dialog,
  HStack,
  IconButton,
  Input,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import MoneyInput from "../../components/MoneyInput";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { usePayPurchaseMutation } from "../../queries/purchasing";

// Record a payment against a purchase order. Page-local, and stays MOUNTED
// driven by its `open` prop — never `{open && <Dialog…>}` or an early
// `return null`, which would strand Ark's body lock and freeze the page.

export function PayDialog({
  open,
  onClose,
  poId,
  outstanding,
}: {
  open: boolean;
  onClose: () => void;
  poId: string;
  outstanding: number;
}) {
  const { t } = useTranslation();
  const payMut = usePayPurchaseMutation();
  const [amount, setAmount] = useState(String(outstanding));
  const [note, setNote] = useState("");

  const submit = async () => {
    const n = parseInt(amount, 10);
    if (!n || n <= 0) return;
    try {
      await payMut.mutateAsync({
        purchaseOrderId: poId,
        amount: BigInt(n),
        note,
      });
      toast.success(t("purchasing.actions.pay") + " ✓");
      onClose();
    } catch {
      /* */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.payTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Box>
                  <Text fontSize="xs" color="fg.muted">
                    {t("purchasing.outstanding")}
                  </Text>
                  <Text fontSize="md" fontFamily="mono">
                    {formatMoney(outstanding)}
                  </Text>
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.amount")}
                  </Text>
                  <MoneyInput value={amount} onChange={setAmount} />
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.note")}
                  </Text>
                  <Input value={note} onChange={(e) => setNote(e.target.value)} />
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={submit} loading={payMut.isPending}>
                  {t("purchasing.actions.pay")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
