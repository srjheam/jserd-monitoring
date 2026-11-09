"use client";

import { NextIntlClientProvider } from "next-intl";
import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { I18nProvider } from "react-aria-components/I18nProvider";
import { type AppLocale, defaultLocale, localeMessages } from "../i18n/locales";

type ClientProvidersProps = {
  children: ReactNode;
};

// The locale lives in client state so the switcher never reloads the page.
const SetLocaleContext = createContext<(locale: AppLocale) => void>(() => {});

export function useSetLocale() {
  return useContext(SetLocaleContext);
}

export function ClientProviders({ children }: ClientProvidersProps) {
  const [locale, setLocale] = useState<AppLocale>(defaultLocale);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  return (
    <SetLocaleContext value={setLocale}>
      {/* No message formats a date, and a fixed zone keeps the prerendered
          markup equal to the hydrated markup. */}
      <NextIntlClientProvider
        locale={locale}
        messages={localeMessages[locale]}
        timeZone="UTC"
      >
        <I18nProvider locale={locale}>{children}</I18nProvider>
      </NextIntlClientProvider>
    </SetLocaleContext>
  );
}
