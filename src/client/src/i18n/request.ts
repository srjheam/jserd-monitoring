import { getRequestConfig } from "next-intl/server";
import { defaultLocale, localeMessages } from "./locales";

// The exported build is static, so the server side only ever renders the
// default locale. The switcher runs in the browser, in ClientProviders.
export default getRequestConfig(async () => ({
  locale: defaultLocale,
  messages: localeMessages[defaultLocale],
}));
