import { Button, HStack, IconButton, Stack, Text } from "@chakra-ui/react";
import { Camera, Trash2 } from "lucide-react";
import { useRef, useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import UserAvatar from "../../components/UserAvatar";
import { ACCEPT_ATTR, ImageRenditionError } from "../../lib/imageRenditions";
import { toast } from "../../lib/toaster";
import { useDeleteAvatarMutation, useUploadAvatarMutation } from "../../queries/users";

type Props = {
  userId: string;
  name: string;
  /** The user's avatarUpdatedAt. 0 = no picture yet. */
  version: number;
};

// Profile picture + its Change/Remove actions. The two renditions (original +
// thumbnail) are produced inside useUploadAvatarMutation, so this component
// only ever hands over the raw File — see the two-rendition HARD RULE.
export default function AvatarPicker({ userId, name, version }: Props) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const upload = useUploadAvatarMutation();
  const remove = useDeleteAvatarMutation();

  const onFileChange = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    // Reset immediately so re-picking the SAME file fires change again.
    e.target.value = "";
    if (!file) return;
    try {
      await upload.mutateAsync(file);
      toast.success(t("profile.avatarUpdated"));
    } catch (err) {
      // Client-side rejections (wrong type, too big, undecodable) carry their
      // own i18n key; anything else is a server error and goes through the
      // shared translator.
      if (err instanceof ImageRenditionError) toast.error(t(err.i18nKey));
      else toast.fromError(err);
    }
  };

  return (
    <Stack gap={2} align="center">
      <UserAvatar userId={userId} name={name} version={version} size="2xl" />
      <HStack gap={1}>
        <Button
          size="xs"
          variant="outline"
          onClick={() => inputRef.current?.click()}
          loading={upload.isPending}
        >
          <Camera size={13} />
          {t(version > 0 ? "profile.avatarChange" : "profile.avatarUpload")}
        </Button>
        {version > 0 && (
          <IconButton
            size="xs"
            variant="ghost"
            aria-label={t("profile.avatarRemove")}
            onClick={() => setConfirmRemove(true)}
            loading={remove.isPending}
          >
            <Trash2 size={13} />
          </IconButton>
        )}
      </HStack>
      <Text fontSize="2xs" color="fg.muted" textAlign="center" maxW="180px">
        {t("profile.avatarHelp")}
      </Text>

      {/* Hidden native input: a file picker is OS chrome by necessity, so this
          is not a "no native dialogs" violation (same call as ImportProducts). */}
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTR}
        style={{ display: "none" }}
        onChange={onFileChange}
      />

      <ConfirmDialog
        open={confirmRemove}
        title={t("profile.avatarRemove")}
        body={t("profile.avatarRemoveConfirm")}
        confirmLabel={t("common.delete")}
        loading={remove.isPending}
        onCancel={() => setConfirmRemove(false)}
        onConfirm={async () => {
          await remove.mutateAsync();
          setConfirmRemove(false);
        }}
      />
    </Stack>
  );
}
