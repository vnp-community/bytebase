// i18n: i18next | use t("key") from useTranslation()
// AI: Copy + rename for CREATE sheets (creating new entities).
// Simpler than edit: no useRef freeze, no entity key, no updateMask.
//
// RULES:
//   1. Create button disabled until all required fields are filled
//   2. Reset form on successful creation
//   3. No updateMask needed — server creates with all provided fields

import { useState, useCallback } from "react";

// TODO: Import sheet UI components
// import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from "@/react/components/ui/sheet";
// import { Button } from "@/react/components/ui/button";

interface Props {
  open: boolean;
  onClose: () => void;
  /** Parent resource name (e.g., "projects/my-project") */
  parent: string;
}

export function TemplateCreateSheet({ open, onClose, parent }: Props) {
  // Form state
  const [title, setTitle] = useState("");
  // TODO: Add more fields

  // Validation
  const isValid = title.trim().length > 0; // TODO: Add more validation

  // TODO: Import mutation hook
  // const { mutate: createEntity, isPending } = useCreateEntity();

  const handleCreate = useCallback(() => {
    if (!isValid) return;

    // TODO: Call mutation
    // createEntity(
    //   { parent, entity: { title, ...otherFields } },
    //   {
    //     onSuccess: () => {
    //       setTitle("");  // Reset form
    //       onClose();
    //     },
    //   }
    // );
  }, [isValid, title, parent, onClose]);

  if (!open) return null;

  return (
    <div className="flex flex-col gap-4 p-4">
      {/* TODO: Replace with SheetHeader */}
      <h2 className="text-lg font-semibold">Create New TODO:EntityName</h2>

      {/* Form fields */}
      <div className="flex flex-col gap-3">
        <label className="text-sm font-medium">Title *</label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Enter title..."
          className="rounded border px-3 py-2"
        />
        {/* TODO: Add more form fields */}
      </div>

      {/* Footer */}
      <div className="flex justify-end gap-2 pt-4">
        <button onClick={onClose}>Cancel</button>
        <button
          onClick={handleCreate}
          disabled={!isValid /* || isPending */}
          className="bg-accent text-white rounded px-4 py-2 disabled:opacity-50"
        >
          Create
        </button>
      </div>
    </div>
  );
}
