"use client";

import { Inbox } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button, Tooltip, TooltipTrigger } from "react-aria-components";
import type { Submission } from "../../lib/monitoring";

// Reverses "mark up to date": the row and the panel both call this, the same
// as `OnMarkUpToDate`, so the action stays reachable while its own row is
// active and hides its row action.
export type OnSendBackToWork = (submission: Submission) => Promise<void>;

export function SendBackToWorkButton({
  submission,
  onSendBackToWork,
}: {
  submission: Submission;
  onSendBackToWork: OnSendBackToWork;
}) {
  const t = useTranslations("sideScreens");
  return (
    <TooltipTrigger delay={0}>
      <Button
        className="row__send-back"
        aria-label={t("sendBackToWork")}
        onPress={() => void onSendBackToWork(submission)}
      >
        <Inbox size={16} />
      </Button>
      <Tooltip className="tooltip">{t("sendBackToWork")}</Tooltip>
    </TooltipTrigger>
  );
}
