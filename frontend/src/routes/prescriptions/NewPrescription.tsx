import { Box, Button, HStack } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import PageHeader from "../../components/PageHeader";
import { toast } from "../../lib/toaster";
import { useCreatePrescriptionMutation } from "../../queries/prescriptions";
import PrescriptionFormFields, {
  emptyRxForm,
  rxFormCanSubmit,
  rxFormToPayload,
  type RxFormState,
} from "./PrescriptionFormFields";

// Full-page create-resep form. Reached from /prescriptions ("Add") and from POS
// ("Buat resep baru") with ?returnTo=pos&patient=<id> — on success it routes
// back to /pos?attachRx=<newId> so the in-progress cart auto-attaches the resep.
export default function NewPrescription() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const returnTo = params.get("returnTo");
  const patientId = params.get("patient") ?? "";

  const createMut = useCreatePrescriptionMutation();
  const [form, setForm] = useState<RxFormState>(() => ({
    ...emptyRxForm(),
    customerId: patientId,
  }));

  const onChange = (patch: Partial<RxFormState>) =>
    setForm((cur) => ({ ...cur, ...patch }));

  const submit = async () => {
    try {
      const res = await createMut.mutateAsync(rxFormToPayload(form));
      toast.success(t("common.create") + " ✓");
      const newId = res.prescription?.id;
      if (returnTo === "pos" && newId) {
        navigate(`/pos?attachRx=${newId}`);
      } else {
        navigate("/prescriptions");
      }
    } catch {
      /* toast handled globally */
    }
  };

  const cancel = () => navigate(returnTo === "pos" ? "/pos" : "/prescriptions");

  return (
    <Box>
      <BackButton to={returnTo === "pos" ? "/pos" : "/prescriptions"} />
      <PageHeader
        breadcrumbs={[
          { label: t("prescriptions.title"), to: "/prescriptions" },
          { label: t("prescriptions.addTitle") },
        ]}
        title={t("prescriptions.addTitle")}
      />

      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5}>
        <PrescriptionFormFields value={form} onChange={onChange} allowCreatePatient />
        <HStack justify="flex-end" gap={2} pt={4} mt={2} borderTopWidth="1px">
          <Button variant="ghost" onClick={cancel}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={createMut.isPending}
            disabled={!rxFormCanSubmit(form)}
          >
            {t("common.create")}
          </Button>
        </HStack>
      </Box>
    </Box>
  );
}
