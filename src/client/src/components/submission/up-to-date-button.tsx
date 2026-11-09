"use client";

import { Check, ChevronDown } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";
import {
  Button,
  Label,
  Menu,
  MenuItem,
  MenuTrigger,
  Popover,
  TextArea,
  TextField,
  Tooltip,
  TooltipTrigger,
} from "react-aria-components";
import type { Submission } from "../../lib/monitoring";
import { addMonth, isoDate } from "../../lib/work-list";
import { AppModal } from "../modal";
import { SaveStatus, useSaveStatus } from "../save-status";

// The one handler `UpToDateSplitButton` calls, shared by the list row and
// the detail panel it also renders in.
export type OnMarkUpToDate = (
  submission: Submission,
  until: string,
  reason: string,
) => Promise<void>;

const presets = ["oneWeek", "twoWeeks", "oneMonth"] as const;

// The preset names a date on the chief editor's own calendar, so it is
// counted and written in local fields from end to end.
const presetDate = (preset: (typeof presets)[number]) => {
  const date = new Date();
  if (preset === "oneWeek") date.setDate(date.getDate() + 7);
  if (preset === "twoWeeks") date.setDate(date.getDate() + 14);
  return isoDate(preset === "oneMonth" ? addMonth(date) : date);
};

// One control at three depths: the button marks a submission up to date at
// once, and the caret opens "Add a reason" and "Return on…". All three write
// one journal entry. Shared between the list row and the detail panel, where
// it renders next to "Post" for the submission currently open.
export function UpToDateSplitButton({
  submission,
  onMarkUpToDate,
}: {
  submission: Submission;
  onMarkUpToDate: OnMarkUpToDate;
}) {
  const t = useTranslations("workList");
  const tCommon = useTranslations("common");
  const [reasonOpen, setReasonOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [status, setStatus] = useSaveStatus();
  const [error, setError] = useState("");
  return (
    <div className="split-button">
      <TooltipTrigger delay={0}>
        <Button
          className="split-button__main"
          aria-label={t("markUpToDate")}
          onPress={() => void onMarkUpToDate(submission, "", "")}
        >
          <Check size={16} />
        </Button>
        <Tooltip className="tooltip">{t("markUpToDate")}</Tooltip>
      </TooltipTrigger>
      <TooltipTrigger delay={0}>
        <MenuTrigger>
          <Button className="split-button__caret" aria-label={t("moreOptions")}>
            <ChevronDown size={16} />
          </Button>
          <Popover>
            <Menu
              onAction={(key) => {
                if (key === "reason") setReasonOpen(true);
                else
                  void onMarkUpToDate(
                    submission,
                    presetDate(key as (typeof presets)[number]),
                    "",
                  );
              }}
            >
              <MenuItem id="reason">{t("addReason")}</MenuItem>
              {presets.map((preset) => (
                <MenuItem key={preset} id={preset}>
                  {t(`returnOn.${preset}`)}
                </MenuItem>
              ))}
            </Menu>
          </Popover>
        </MenuTrigger>
        <Tooltip className="tooltip">{t("moreOptionsHint")}</Tooltip>
      </TooltipTrigger>
      <AppModal
        isOpen={reasonOpen}
        onOpenChange={setReasonOpen}
        dialogProps={{ "aria-label": t("addReason") }}
      >
        <TextField value={reason} onChange={setReason} autoFocus>
          <Label>{t("reasonLabel")}</Label>
          <TextArea />
        </TextField>
        <SaveStatus
          status={status}
          savingLabel={tCommon("saving")}
          savedLabel={tCommon("saved")}
        />
        {error && (
          <p className="failure" role="alert">
            {error}
          </p>
        )}
        <Button
          isDisabled={status === "loading"}
          onPress={async () => {
            setStatus("loading");
            try {
              await onMarkUpToDate(submission, "", reason);
              setError("");
              setStatus("success");
              setReasonOpen(false);
              setReason("");
            } catch {
              setError(t("markUpToDateFailed"));
              setStatus("error");
            }
          }}
        >
          {status === "loading" ? tCommon("saving") : t("markUpToDate")}
        </Button>
      </AppModal>
    </div>
  );
}
