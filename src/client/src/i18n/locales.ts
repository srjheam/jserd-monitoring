import en from "../../messages/en.json";
import ptBR from "../../messages/pt-BR.json";

export const defaultLocale = "en";

export const localeMessages = { en, "pt-BR": ptBR };

export type AppLocale = keyof typeof localeMessages;

export const locales = Object.keys(localeMessages) as AppLocale[];

export const localeNames: Record<AppLocale, string> = {
  en: "English",
  "pt-BR": "Português",
};
