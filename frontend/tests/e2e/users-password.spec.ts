import { expect, test } from "./_helpers";

// Change password: shared <ChangePasswordDialog> opened from the TopBar user
// menu (self path). Backend ChangePassword (users.go:201-240) already covers
// both self + OWNER-changes-other branches; this spec verifies the UI wiring.
//
// We don't actually Save here: the seeded bootstrap password "test123" is 7
// chars, below the new-password minimum (8), so a round-trip rotate → restore
// can't preserve the seed. Verifying the dialog open/fields/validation/close
// pins the UI without leaving the user in a rotated state.

test.describe("change password", () => {
  test("TopBar menu opens the password dialog with current/new/confirm fields", async ({
    page,
  }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "user menu" }).click();
    await page.getByRole("menu").getByText(/Change my password/i).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    // Three password inputs: current, new, confirm.
    const inputs = dialog.locator('input[type="password"]');
    await expect(inputs).toHaveCount(3);

    // The app does NOT gate Save on form validity — per the validation HARD
    // RULE it runs the Zod schema on submit and <FormField> renders the
    // message under the offending field. Submitting empty keeps the dialog
    // open and surfaces a required error.
    const save = dialog.getByRole("button", { name: "Save" });
    await save.click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(/required|Wajib diisi/i).first()).toBeVisible();

    // Mismatched new/confirm → the mismatch message on submit.
    await inputs.nth(0).fill("test123");
    await inputs.nth(1).fill("abcd1234");
    await inputs.nth(2).fill("zzzz9999");
    await save.click();
    await expect(dialog.getByText(/Passwords do not match|tidak cocok/i)).toBeVisible();

    // Fix the confirm → re-validating clears the mismatch. We deliberately do
    // NOT submit a valid form: that would rotate the seeded password.
    await inputs.nth(2).fill("abcd1234");
    await expect(dialog.getByText(/Passwords do not match|tidak cocok/i)).toHaveCount(0);

    // Cancel — no rotation happens, dialog closes.
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).not.toBeVisible();
  });
});
