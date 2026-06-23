import {
  Button,
  Dialog,
  HStack,
  IconButton,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { X } from "lucide-react";
import { useEffect, useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import FormField from "./FormField";
import { useServerFormErrors } from "../lib/formErrors";
import { toast } from "../lib/toaster";
import { useChangePasswordMutation } from "../queries/users";

type FormValues = { current: string; next: string; confirm: string };

// `current` is required only for the self path; `confirm` must match `next`.
// Custom refines carry their i18n key via params.i18n (resolved by zodErrorMap).
function makeSchema(isSelf: boolean) {
  return z
    .object({
      current: z.string(),
      next: z.string().min(8),
      confirm: z.string(),
    })
    .superRefine((v, ctx) => {
      if (isSelf && v.current.trim().length < 1) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["current"], params: { i18n: "validation.required" } });
      }
      if (v.confirm !== v.next) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["confirm"], params: { i18n: "validation.passwordMismatch" } });
      }
    });
}

// Shared dialog for both "change my password" (TopBar menu) and "OWNER changes
// another user's password" (Users admin). The backend ChangePassword RPC
// already supports both paths: empty user_id => self (requires old_password),
// non-empty user_id => OWNER only (no old_password required).
export default function ChangePasswordDialog({
  open,
  onClose,
  userId,
  isSelf,
  userLabel,
}: {
  open: boolean;
  onClose: () => void;
  userId?: string;
  isSelf: boolean;
  userLabel?: string;
}) {
  const { t } = useTranslation();
  const change = useChangePasswordMutation();
  const schema = useMemo(() => makeSchema(isSelf), [isSelf]);
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { current: "", next: "", confirm: "" },
  });
  const onServerError = useServerFormErrors(form);

  // Reset fields whenever the dialog reopens (different user, or after success).
  useEffect(() => {
    if (open) form.reset({ current: "", next: "", confirm: "" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      await change.mutateAsync({
        userId: isSelf ? "" : userId ?? "",
        oldPassword: isSelf ? v.current : "",
        newPassword: v.next,
      });
      toast.success(t("users.changePassword") + " ✓");
      onClose();
    } catch (err) {
      onServerError(err); // auth.current_password_wrong → field error on `current`
    }
  });

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()} size="sm">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>
                {isSelf ? t("users.changeMyPassword") : t("users.changePassword")}
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                {!isSelf && userLabel && (
                  <Text fontSize="sm" color="fg.muted">
                    {userLabel}
                  </Text>
                )}
                {isSelf && (
                  <FormField
                    control={form.control}
                    name="current"
                    label={t("users.currentPassword")}
                    type="password"
                    passwordToggle
                    autoFocus
                    required
                  />
                )}
                <FormField
                  control={form.control}
                  name="next"
                  label={t("users.newPassword")}
                  type="password"
                  passwordToggle
                  autoFocus={!isSelf}
                  required
                />
                <FormField
                  control={form.control}
                  name="confirm"
                  label={t("users.confirmPassword")}
                  type="password"
                  passwordToggle
                  required
                />
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={onSubmit} loading={change.isPending}>
                  {t("common.save")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
