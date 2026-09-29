import { useState } from 'react';

export interface ManifestViewProps {
  yaml: string | null;
  error: string | null;
  /** Enables editing; resolves when the edited manifest was applied */
  onSave?: (yaml: string) => Promise<void>;
}

export const useManifestView = ({ yaml, error, onSave }: ManifestViewProps) => {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const startEditing = () => {
    setDraft(yaml || '');
    setSaveError(null);
    setEditing(true);
  };

  const save = async () => {
    if (!onSave) return;
    setSaving(true);
    setSaveError(null);
    try {
      await onSave(draft);
      setEditing(false);
    } catch (err: any) {
      setSaveError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return {
    yaml,
    error,
    editable: !!onSave && yaml !== null,
    editing,
    draft,
    setDraft,
    saving,
    saveError,
    startEditing,
    cancel: () => setEditing(false),
    save,
  };
};
