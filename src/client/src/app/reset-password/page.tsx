"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import {
  Button,
  FieldError,
  Form,
  Heading,
  Input,
  Label,
  TextField,
} from "react-aria-components";
import type { SaveState } from "../../components/save-status";
import { pb } from "../../lib/api";

// The token comes from the link in PocketBase's password-reset e-mail (see
// the chief_editors reset template in the server migrations), read straight
// from the query string, matching how the rest of this SPA reads page state
// (see readPageLocation in lib/monitoring.ts) instead of Next's router hooks.
export default function ResetPassword() {
  const t = useTranslations("passwordReset");
  const tCommon = useTranslations("common");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useState<SaveState>("idle");
  const token =
    typeof window === "undefined"
      ? ""
      : (new URLSearchParams(window.location.search).get("token") ?? "");

  if (!token)
    return (
      <main className="sign-in">
        <div>
          <Heading level={1}>{t("confirmTitle")}</Heading>
          <p className="failure" role="alert">
            {t("missingToken")}
          </p>
          <a href="/">{t("goToSignIn")}</a>
        </div>
      </main>
    );

  return (
    <main className="sign-in">
      <Form
        validationBehavior="aria"
        onSubmit={async (event) => {
          event.preventDefault();
          setStatus("loading");
          try {
            await pb
              .collection("chief_editors")
              .confirmPasswordReset(token, password, passwordConfirm);
            setStatus("success");
          } catch {
            setError(t("confirmFailed"));
            setStatus("error");
          }
        }}
      >
        <Heading level={1}>{t("confirmTitle")}</Heading>
        {status === "success" ? (
          <>
            <output>{t("confirmSuccess")}</output>
            <a href="/">{t("goToSignIn")}</a>
          </>
        ) : (
          <>
            <p>{t("confirmSubtitle")}</p>
            <TextField value={password} onChange={setPassword} isRequired>
              <Label>{t("newPassword")}</Label>
              <Input type="password" />
              <FieldError />
            </TextField>
            <TextField
              value={passwordConfirm}
              onChange={setPasswordConfirm}
              isRequired
            >
              <Label>{t("confirmPassword")}</Label>
              <Input type="password" />
              <FieldError />
            </TextField>
            {error && (
              <p className="failure" role="alert">
                {error}
              </p>
            )}
            <Button type="submit" isDisabled={status === "loading"}>
              {status === "loading" ? tCommon("saving") : t("confirmSubmit")}
            </Button>
          </>
        )}
      </Form>
    </main>
  );
}
