import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import { strings, type Locale, type StringKey } from "./strings";

const STORAGE_KEY = "lang";
const DEFAULT_LOCALE: Locale = "zh";

function readStoredLocale(): Locale {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "zh" || v === "en") return v;
  } catch {
    // localStorage 可能被禁用
  }
  return DEFAULT_LOCALE;
}

interface LangContextValue {
  locale: Locale;
  toggleLocale: () => void;
  t: (key: StringKey, ...args: string[]) => string;
}

const LangContext = createContext<LangContextValue | null>(null);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocale] = useState<Locale>(readStoredLocale);

  const toggleLocale = useCallback(() => {
    setLocale((prev) => {
      const next: Locale = prev === "zh" ? "en" : "zh";
      try {
        localStorage.setItem(STORAGE_KEY, next);
      } catch {
        // 忽略
      }
      return next;
    });
  }, []);

  const t = useCallback(
    (key: StringKey, ...args: string[]): string => {
      const dict = strings[locale];
      const val = dict[key];
      if (typeof val === "function") {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        return (val as any)(...args) as string;
      }
      return val as string;
    },
    [locale],
  );

  const value = useMemo(
    () => ({ locale, toggleLocale, t }),
    [locale, toggleLocale, t],
  );

  return <LangContext.Provider value={value}>{children}</LangContext.Provider>;
}

export function useLang(): LangContextValue {
  const ctx = useContext(LangContext);
  if (!ctx) throw new Error("useLang must be used inside LanguageProvider");
  return ctx;
}

// 简写 hook，只需要 t 函数时用
export function useT(): LangContextValue["t"] {
  return useLang().t;
}
