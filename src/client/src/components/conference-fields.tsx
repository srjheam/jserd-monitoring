"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { FieldError, Input, Label, TextField } from "react-aria-components";
import type { SaveState } from "./save-status";

// Two forms create a conference: the manage-conferences dialog in
// monitoring-dashboard.tsx and the picker in the submission panel. They share
// the same two fields, the same `conferencesForm` translation namespace, the same
// status handling and the same error string. A rule added to one form must
// reach both, or the picker can show a server error the dialog cannot explain.
//
// The dialog also edits and lists conferences. The picker has a Back button.

// ConferenceDraft holds the fields and save status for the current form.
export type ConferenceDraft = {
  name: string;
  description: string;
  error: string;
  status: SaveState;
  setName: (value: string) => void;
  setDescription: (value: string) => void;
  // clear resets the fields after a save. The dialog stays open; the picker
  // closes over the form.
  clear: () => void;
  // setError shows a message without marking a save as failed. The dialog uses
  // it for a failed delete, which is not a save of the draft at all.
  setError: (message: string) => void;
  // fail records a failed save: the message both forms show, and the status.
  fail: (message: string) => void;
  // start marks the save as running, which disables the submit button.
  start: () => void;
};

export function useConferenceDraft(): ConferenceDraft {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useState<SaveState>("idle");
  return {
    name,
    description,
    error,
    status,
    setName,
    setDescription,
    setError,
    clear: () => {
      setName("");
      setDescription("");
      setError("");
    },
    fail: (message: string) => {
      setError(message);
      setStatus("error");
    },
    start: () => setStatus("loading"),
  };
}

// ConferenceFields is the name, the description and the error line. Both forms place
// it inside their own Form, with their own submit button, because the button's
// words differ: the dialog says Update while it edits an existing conference.
export function ConferenceFields({ draft }: { draft: ConferenceDraft }) {
  const t = useTranslations("conferencesForm");
  return (
    <>
      <TextField value={draft.name} onChange={draft.setName} isRequired>
        <Label>{t("name")}</Label>
        <Input />
        <FieldError />
      </TextField>
      <TextField value={draft.description} onChange={draft.setDescription}>
        <Label>{t("description")}</Label>
        <Input />
      </TextField>
      {draft.error && <p className="failure">{draft.error}</p>}
    </>
  );
}
