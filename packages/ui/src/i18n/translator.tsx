import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { failureCode } from "../failures.ts";
import type { LanguageSource } from "../vault-api.ts";
import { LanguageFollower } from "./follower.ts";
import { type MessageValues, messageFormatter } from "./format.ts";
import {
  isLanguage,
  type Language,
  languageNames,
  languages,
} from "./language.ts";
import { catalogs, type MessageKey } from "./messages.ts";

export interface Translator {
  language: Language;
  languages: readonly Language[];
  t: (key: MessageKey, values?: MessageValues) => string;
  /** failure explains an error the host reported by its code, or with the fallback message. */
  failure: (
    error: unknown,
    fallback: MessageKey,
    values?: MessageValues,
  ) => string;
  setLanguage: (language: Language) => Promise<void>;
}

const TranslatorContext = createContext<Translator | null>(null);

function isMessageKey(key: string): key is MessageKey {
  return key in catalogs.en;
}

export function LanguageProvider({
  api,
  initial = "en",
  children,
}: {
  api?: LanguageSource;
  /** The language shown until the source answers. */
  initial?: Language;
  children: ReactNode;
}) {
  const [language, setLanguage] = useState<Language>(initial);

  useEffect(() => {
    if (!api) return;
    return new LanguageFollower(api).follow(setLanguage);
  }, [api]);

  const choose = useCallback(
    async (next: Language) => {
      setLanguage(next);
      await api?.setLanguage?.(next);
    },
    [api],
  );

  const translator = useMemo<Translator>(() => {
    const t = (key: MessageKey, values?: MessageValues) =>
      messageFormatter.format(language, key, values);
    return {
      language,
      languages,
      t,
      failure: (error, fallback, values) => {
        const reported = failureCode(error);
        const code = `failure.${reported}`;
        return t(reported && isMessageKey(code) ? code : fallback, values);
      },
      setLanguage: choose,
    };
  }, [language, choose]);

  return (
    <TranslatorContext.Provider value={translator}>
      {children}
    </TranslatorContext.Provider>
  );
}

export function useTranslator(): Translator {
  const translator = useContext(TranslatorContext);
  if (!translator) {
    throw new Error("Ravenpass is missing its language provider.");
  }
  return translator;
}

export interface LanguageOption {
  value: Language;
  label: string;
}

/** LanguageChoice is what every language switcher shows and does. */
export interface LanguageChoice {
  language: Language;
  options: readonly LanguageOption[];
  /** choose ignores a value that names no offered language. */
  choose: (value: string) => void;
}

export function useLanguageChoice(): LanguageChoice {
  const { language, languages, setLanguage } = useTranslator();
  return {
    language,
    options: languages.map((value) => ({ value, label: languageNames[value] })),
    choose: (value) => {
      if (isLanguage(value)) void setLanguage(value);
    },
  };
}
