import { Toaster } from "@ravenpass/ui/components/ui/sonner.tsx";
import {
  LanguageProvider,
  useTranslator,
} from "@ravenpass/ui/i18n/translator.tsx";
import { MotionProvider } from "@ravenpass/ui/motion/MotionProvider.tsx";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { StoredLanguage } from "../language.ts";
import { ask } from "../messages.ts";
import { LinkedScreen, type LinkedStatus } from "./LinkedScreen.tsx";
import { LinkScreen } from "./LinkScreen.tsx";

const language = new StoredLanguage();

type Screen =
  | { name: "loading" }
  | { name: "link" }
  | { name: "linked"; status: LinkedStatus };

export function Popup() {
  return (
    <LanguageProvider api={language}>
      <MotionProvider>
        <Screens />
      </MotionProvider>
      <Toaster position="bottom-center" />
    </LanguageProvider>
  );
}

function Screens() {
  const { t } = useTranslator();
  const [screen, setScreen] = useState<Screen>({ name: "loading" });

  // The status arrives after the popup opens; the notice reads the language in use by then.
  const translate = useRef(t);
  translate.current = t;

  const refresh = useCallback(async () => {
    // A status the service worker cannot give shows the link screen.
    const status = await ask({ kind: "status" }).catch(() => null);
    if (status?.linked) {
      setScreen({ name: "linked", status });
      return;
    }
    if (status?.unlinkedInRavenpass)
      toast(translate.current("extension.unlinked"));
    setScreen({ name: "link" });
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  switch (screen.name) {
    case "loading":
      return null;
    case "link":
      return <LinkScreen onLinked={refresh} />;
    case "linked":
      return (
        <LinkedScreen
          status={screen.status}
          onUnlinked={() => setScreen({ name: "link" })}
        />
      );
  }
}
