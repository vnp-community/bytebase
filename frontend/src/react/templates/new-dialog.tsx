// i18n: i18next | use t("key") from useTranslation()
// AI: Copy + rename for confirmation dialogs.
// For destructive actions, use AlertDialog pattern (red button).
//
// RULES:
//   1. Use AlertDialog for destructive actions (delete, revoke, etc.)
//   2. Use Dialog for non-destructive confirmations
//   3. Never use document.body for portals — use overlay layer system
//   4. Always include accessible title and description

// TODO: Import dialog components
// import {
//   AlertDialog, AlertDialogAction, AlertDialogCancel,
//   AlertDialogContent, AlertDialogDescription, AlertDialogFooter,
//   AlertDialogHeader, AlertDialogTitle,
// } from "@/react/components/ui/alert-dialog";

interface Props {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  /** Whether the action is in progress */
  loading?: boolean;
}

/**
 * Confirmation dialog for destructive actions.
 * TODO: Replace with your specific confirmation.
 */
export function TemplateConfirmDialog({
  open,
  onClose,
  onConfirm,
  loading = false,
}: Props) {
  if (!open) return null;

  return (
    // TODO: Replace with AlertDialog component
    <div
      role="alertdialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        {/* Header */}
        <h2 className="text-lg font-semibold text-main-text">
          TODO: Confirm Action
        </h2>
        <p className="mt-2 text-sm text-control-placeholder">
          TODO: Are you sure you want to perform this action? This cannot be
          undone.
        </p>

        {/* Footer */}
        <div className="mt-6 flex justify-end gap-2">
          <button
            onClick={onClose}
            disabled={loading}
            className="rounded px-4 py-2 text-sm"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={loading}
            className="rounded bg-red-600 px-4 py-2 text-sm text-white hover:bg-red-700 disabled:opacity-50"
          >
            {loading ? "Deleting..." : "Delete"}
          </button>
        </div>
      </div>
    </div>
  );
}
