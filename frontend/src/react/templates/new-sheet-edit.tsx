// i18n: i18next | use t("key") from useTranslation()
// AI: Copy + rename for EDIT sheets (editing existing entities).
// This implements the outer + inner + key pattern:
//   - Outer: holds stable ref, passes entity to Inner via key
//   - Inner: resets form state when entity changes (via key prop)
//   - isDirty: enables/disables Update button
//
// RULES:
//   1. useRef to freeze entity reference for stable comparison
//   2. key={entity.name} on Inner to reset form when switching entities
//   3. Update button disabled until isDirty === true
//   4. Always pass updateMask with only changed fields

import { useRef, useState, useCallback } from "react";

// TODO: Import sheet UI components
// import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from "@/react/components/ui/sheet";
// import { Button } from "@/react/components/ui/button";

// TODO: Replace with your entity type
interface TemplateEntity {
  name: string;
  title: string;
  // ...other fields
}

interface OuterProps {
  open: boolean;
  onClose: () => void;
  /** Entity to edit — passed from parent */
  entity: TemplateEntity | null;
}

/** Outer: Holds stable entity ref, renders Inner with key */
export function TemplateEditSheet({ open, onClose, entity }: OuterProps) {
  // Freeze entity ref when sheet opens — prevents re-render from parent updates
  const stableRef = useRef(entity);
  if (open && entity) {
    stableRef.current = entity;
  }
  const stableEntity = stableRef.current;

  if (!stableEntity) return null;

  return (
    // TODO: Wrap with Sheet component
    <div>
      {/* key resets Inner state when switching between entities */}
      <TemplateEditSheetInner
        key={stableEntity.name}
        entity={stableEntity}
        onClose={onClose}
      />
    </div>
  );
}

interface InnerProps {
  entity: TemplateEntity;
  onClose: () => void;
}

/** Inner: Contains form state, isDirty tracking, save logic */
function TemplateEditSheetInner({ entity, onClose }: InnerProps) {
  // Form state — initialized from entity
  const [title, setTitle] = useState(entity.title);
  // TODO: Add more form fields

  // Track if form has unsaved changes
  const isDirty = title !== entity.title; // TODO: Add all fields
  const isValid = title.trim().length > 0; // TODO: Add validation

  // TODO: Import and use your mutation hook
  // const { mutate: updateEntity, isPending } = useUpdateEntity();

  const handleSave = useCallback(() => {
    if (!isDirty || !isValid) return;

    // TODO: Build updateMask from changed fields only
    const updateMask: string[] = [];
    if (title !== entity.title) updateMask.push("title");
    // TODO: Add more field checks

    // TODO: Call mutation
    // updateEntity(
    //   { entity: { ...entity, title }, updateMask },
    //   { onSuccess: () => onClose() }
    // );
  }, [isDirty, isValid, title, entity, onClose]);

  return (
    <div className="flex flex-col gap-4 p-4">
      {/* TODO: Replace with SheetHeader */}
      <h2 className="text-lg font-semibold">Edit: {entity.name}</h2>

      {/* TODO: Form fields */}
      <div className="flex flex-col gap-3">
        <label className="text-sm font-medium">Title</label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className="rounded border px-3 py-2"
        />
      </div>

      {/* TODO: Replace with SheetFooter + Button */}
      <div className="flex justify-end gap-2 pt-4">
        <button onClick={onClose}>Cancel</button>
        <button
          onClick={handleSave}
          disabled={!isDirty || !isValid /* || isPending */}
          className="bg-accent text-white rounded px-4 py-2 disabled:opacity-50"
        >
          Update
        </button>
      </div>
    </div>
  );
}
