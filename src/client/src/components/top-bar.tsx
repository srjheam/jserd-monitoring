"use client";

import { useLocale, useTranslations } from "next-intl";
import { useEffect, useState, useSyncExternalStore } from "react";
import {
  Button,
  type Key,
  Menu,
  MenuItem,
  MenuTrigger,
  Popover,
} from "react-aria-components";
import { useSetLocale } from "../app/provider";
import { type AppLocale, localeNames, locales } from "../i18n/locales";
import type { MonitoringMeta } from "../lib/monitoring";

type Translate = ReturnType<typeof useTranslations>;

// Below 768px the locale switcher and the user menu collapse into one button,
// so only one of the two shapes is ever in the DOM.
const compactQuery = "(max-width: 767px)";

export function useCompact() {
  return useSyncExternalStore(
    (onChange) => {
      const query = window.matchMedia(compactQuery);
      query.addEventListener("change", onChange);
      return () => query.removeEventListener("change", onChange);
    },
    () => window.matchMedia(compactQuery).matches,
    () => false,
  );
}

export function TopBar({
  meta,
  user,
  onSignOut,
  onManageConferences,
}: {
  meta: MonitoringMeta | null;
  user: string;
  onSignOut: () => void;
  onManageConferences: () => void;
}) {
  const t = useTranslations("shell");
  const tDashboard = useTranslations("dashboard");
  const locale = useLocale() as AppLocale;
  const setLocale = useSetLocale();
  const compact = useCompact();
  const act = (key: Key) => {
    if (key === "signOut") onSignOut();
    else if (key === "manageConferences") onManageConferences();
    else setLocale(String(key).replace("locale:", "") as AppLocale);
  };
  const localeItems = locales.map((value) => (
    <MenuItem key={value} id={`locale:${value}`}>
      {localeNames[value]}
    </MenuItem>
  ));
  return (
    <header className="topbar">
      <h1>{tDashboard("title")}</h1>
      <CollectionHealth meta={meta} />
      <div className="topbar-shell-actions">
        {!compact && (
          <>
            <MenuTrigger>
              <Button>
                <span aria-hidden="true">🌐 </span>
                <span className="-screenReader">{t("language")}: </span>
                {localeNames[locale]}
              </Button>
              <Popover>
                <Menu
                  onAction={act}
                  selectionMode="single"
                  selectedKeys={[`locale:${locale}`]}
                >
                  {localeItems}
                </Menu>
              </Popover>
            </MenuTrigger>
            <MenuTrigger>
              <Button>
                <span className="-screenReader">{t("account")}: </span>
                {user || t("account")}
              </Button>
              <Popover>
                <Menu onAction={act}>
                  <MenuItem id="signOut">{t("signOut")}</MenuItem>
                </Menu>
              </Popover>
            </MenuTrigger>
          </>
        )}
        <MenuTrigger>
          <Button aria-label={t("menu")}>
            <span aria-hidden="true">☰</span>
          </Button>
          <Popover>
            <Menu onAction={act}>
              <MenuItem id="manageConferences">
                {tDashboard("manageConferences")}
              </MenuItem>
              {compact && localeItems}
              {compact && <MenuItem id="signOut">{t("signOut")}</MenuItem>}
            </Menu>
          </Popover>
        </MenuTrigger>
      </div>
    </header>
  );
}

function CollectionHealth({ meta }: { meta: MonitoringMeta | null }) {
  const t = useTranslations("shell");
  const status = meta?.latestAttempt?.status;
  const dot =
    status === "succeeded"
      ? "health-dot"
      : status
        ? "health-dot health-dot-failed"
        : "health-dot health-dot-unknown";
  return (
    <p className="health">
      <span aria-hidden="true" className={dot} />
      <span className="-screenReader">
        {status === "succeeded"
          ? t("collectionSucceeded")
          : status
            ? t("collectionFailed")
            : t("collectionUnknown")}
      </span>
      <LastSync collection={meta?.latestSuccess} />
    </p>
  );
}

function LastSync({
  collection,
}: {
  collection?: MonitoringMeta["latestSuccess"];
}) {
  const t = useTranslations("dashboard.lastSync");
  const locale = useLocale();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, []);
  const finishedAt =
    collection?.status === "succeeded"
      ? Date.parse(collection.finishedAt)
      : NaN;
  return (
    <output className="last-sync" aria-live="polite">
      {t("label", { time: formatSyncAge(finishedAt, now, locale, t) })}
    </output>
  );
}

function formatSyncAge(
  finishedAt: number,
  now: number,
  locale: string,
  t: Translate,
) {
  if (!Number.isFinite(finishedAt)) return t("unavailable");
  const minutes = Math.floor(Math.max(0, now - finishedAt) / 60_000);
  if (minutes === 0) return t("justNow");
  const relative = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (minutes < 60) return relative.format(-minutes, "minute");
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return relative.format(-hours, "hour");
  return relative.format(-Math.floor(hours / 24), "day");
}
