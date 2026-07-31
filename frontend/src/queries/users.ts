import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { userClient } from "../lib/clients";
import { AvatarVariant } from "../gen/user_iface/v1/users_pb";
import type {
  ChangePasswordRequest,
  CreateUserRequest,
  SetUserActiveRequest,
  UpdateUserRoleRequest,
} from "../gen/user_iface/v1/users_pb";
import { useAuth } from "../lib/auth";
import { dataUrlFromBytes, makeImageRenditions } from "../lib/imageRenditions";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export const userKeys = {
  all: ["users"] as const,
  list: (page: number, pageSize: number) =>
    [...userKeys.all, "list", page, pageSize] as const,
  // Prefix for invalidation: matches every page of the list.
  lists: () => [...userKeys.all, "list"] as const,
};

export const avatarKeys = {
  all: ["avatars"] as const,
  // `version` is the user's avatar_updated_at. Baking it into the key is what
  // makes a re-upload appear without a manual invalidate — new timestamp, new
  // cache entry — and lets the old bytes stay cached forever.
  one: (userId: string, variant: AvatarVariant, version: number) =>
    [...avatarKeys.all, userId, variant, version] as const,
};

// Server-paginated; returns { rows, total }.
export function useUsersQuery(opts: { page?: number; pageSize?: number } = {}) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: userKeys.list(page, pageSize),
    queryFn: async () => {
      const res = await userClient.listUsers({ limit: pageSize, offset: page * pageSize });
      return { rows: res.users, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateUserMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUserRequest>) => userClient.createUser(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.lists() }),
    // The form handles errors via useServerFormErrors (field-level + fallback).
    meta: { silentError: true },
  });
}

export function useUpdateUserRoleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUserRoleRequest>) =>
      userClient.updateUserRole(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.lists() }),
  });
}

export function useSetUserActiveMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetUserActiveRequest>) =>
      userClient.setUserActive(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.lists() }),
  });
}

export function useChangePasswordMutation() {
  return useMutation({
    mutationFn: (req: PartialMessage<ChangePasswordRequest>) =>
      userClient.changePassword(req),
    // The dialog handles errors via useServerFormErrors (field-level + fallback).
    meta: { silentError: true },
  });
}

// Imperative search helper — backend ILIKEs email/name over ACTIVE users only.
// Drives the warehouse-detail "Add user" picker and the resep issuer picker via
// SearchableSelect's loadOptions async mode.
export async function searchUsers(query: string) {
  const res = await userClient.searchUsers({ query, limit: 20 });
  return [...res.users];
}

// Same search narrowed to people with at least one COMPLETED sale — the option
// source for the cashier-scope filter, where an option that can only ever yield
// an empty table is worse than no option. Separate export rather than a flag on
// searchUsers so it drops straight into `loadOptions` without a wrapper closure
// (SearchableSelect keys its debounce on that callback's identity).
export async function searchSellingUsers(query: string) {
  const res = await userClient.searchUsers({ query, limit: 20, withSalesOnly: true });
  return [...res.users];
}

/**
 * A user's profile picture as a ready-to-render data URL ("" when they have
 * none). Defaults to the THUMB rendition — per the two-rendition HARD RULE,
 * only a deliberate full-size view passes ORIGINAL.
 *
 * `version` is the user's `avatarUpdatedAt`; pass 0 and the query is disabled
 * entirely, so a user with no avatar costs zero requests no matter how many
 * rows render them.
 */
export function useAvatarQuery(
  userId: string,
  version: number,
  variant: AvatarVariant = AvatarVariant.THUMB,
) {
  return useQuery({
    queryKey: avatarKeys.one(userId, variant, version),
    queryFn: async () => {
      const res = await userClient.getAvatar({ userId, variant });
      return dataUrlFromBytes(res.imageData, res.contentType);
    },
    enabled: !!userId && version > 0,
    // Immutable by construction: the version in the key changes on re-upload,
    // so a cached entry can never go stale.
    staleTime: Infinity,
    gcTime: 30 * 60_000,
  });
}

/**
 * Upload the caller's own picture. Takes the raw File and does the
 * two-rendition downscale itself, so no call site can forget the thumbnail.
 */
export function useUploadAvatarMutation() {
  const qc = useQueryClient();
  const { refreshUser } = useAuth();
  return useMutation({
    mutationFn: async (file: File) => {
      const { original, thumb, contentType } = await makeImageRenditions(file);
      return userClient.uploadAvatar({
        imageData: original,
        thumbData: thumb,
        contentType,
      });
    },
    onSuccess: async () => {
      // The signed-in user's avatarUpdatedAt IS the cache key every avatar
      // renders from, so re-reading it is what swaps the picture app-wide.
      // Done here rather than at the call site so no caller can forget.
      await refreshUser();
      void qc.invalidateQueries({ queryKey: userKeys.all });
    },
    meta: { silentError: true },
  });
}

export function useDeleteAvatarMutation() {
  const qc = useQueryClient();
  const { refreshUser } = useAuth();
  return useMutation({
    mutationFn: () => userClient.deleteAvatar({}),
    onSuccess: async () => {
      await refreshUser();
      void qc.invalidateQueries({ queryKey: userKeys.all });
    },
  });
}
