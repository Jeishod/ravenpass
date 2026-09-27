import { skipToken, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, LoaderCircle } from "lucide-react";
import { useState } from "react";
import { LockedView } from "./components/LockedView.tsx";
import { SetupWizard } from "./components/SetupWizard.tsx";
import { StorageUnavailableView } from "./components/StorageUnavailableView.tsx";
import { Toaster } from "./components/ui/sonner.tsx";
import { VaultScreen } from "./components/VaultScreen.tsx";
import { VaultView } from "./components/VaultView.tsx";
import { CapabilitiesProvider } from "./host/capabilities.tsx";
import { useCompactLayout } from "./host/compact.ts";
import type { MessageKey } from "./i18n/messages.ts";
import { LanguageProvider, useTranslator } from "./i18n/translator.tsx";
import { IntroView } from "./intro/IntroView.tsx";
import { type OpeningPhase, openingPhase } from "./intro/opening.ts";
import { CrossFade } from "./motion/CrossFade.tsx";
import { MotionProvider } from "./motion/MotionProvider.tsx";
import { rise } from "./motion/timings.ts";
import { forgetVault } from "./query/client.ts";
import { FailureToasts } from "./query/failure-toasts.ts";
import { queryKeys } from "./query/keys.ts";
import type { VaultApi, VaultState } from "./vault-api.ts";

type AppPhase = OpeningPhase | "loading" | "error" | "unavailable";

/** Sonner reads `mobileOffset` below 600px and `offset` above. */
const belowStatusBar = { top: "calc(env(safe-area-inset-top) + 8px)" };

export function App({ api }: { api?: VaultApi }) {
  const client = useQueryClient();
  const opening = useQuery({
    queryKey: queryKeys.opening,
    queryFn: api ? () => openingPhase(api) : skipToken,
  });
  const [chosen, setChosen] = useState<OpeningPhase | null>(null);
  const [introPlayed, setIntroPlayed] = useState(false);
  const [fromIntro, setFromIntro] = useState(false);
  const compact = useCompactLayout();
  const phase: AppPhase = !api
    ? "unavailable"
    : (chosen ?? (opening.isError ? "error" : (opening.data ?? "loading")));

  function setPhase(next: OpeningPhase) {
    if (next !== "ready") forgetVault(client);
    setChosen(next);
  }

  function leaveIntro(next: VaultState["phase"]) {
    setIntroPlayed(true);
    setFromIntro(true);
    setPhase(next);
  }

  function moveOn(next: VaultState["phase"]) {
    setFromIntro(false);
    setPhase(next);
  }

  const backToIntro = fromIntro ? () => setPhase("intro") : undefined;

  return (
    <LanguageProvider api={api}>
      <FailureToasts />
      <CapabilitiesProvider api={api}>
        <MotionProvider>
          <CrossFade id={phase} presence={rise} appear className="h-dvh">
            {screenFor()}
          </CrossFade>
        </MotionProvider>
      </CapabilitiesProvider>
      <Toaster
        position={compact ? "top-center" : "bottom-right"}
        offset={compact ? belowStatusBar : undefined}
        mobileOffset={compact ? belowStatusBar : undefined}
      />
    </LanguageProvider>
  );

  function screenFor() {
    if (phase === "loading") {
      return (
        <CenteredMessage
          title="app.opening.title"
          detail="app.opening.detail"
          loading
        />
      );
    }
    if (phase === "unavailable") {
      return (
        <CenteredMessage
          title="app.unavailable.title"
          detail="app.unavailable.detail"
        />
      );
    }
    if (phase === "error") {
      return (
        <CenteredMessage
          title="app.error.title"
          detail="app.error.detail"
          error={opening.error}
        />
      );
    }
    if (!api) return null;
    if (phase === "intro") {
      return <IntroView api={api} played={introPlayed} onPhase={leaveIntro} />;
    }
    if (phase === "storage") {
      return <StorageUnavailableView api={api} onOpened={setPhase} />;
    }
    if (phase === "setup") {
      return <SetupWizard api={api} onBack={backToIntro} onPhase={moveOn} />;
    }
    if (phase === "locked") {
      return (
        <LockedView
          api={api}
          onPhase={moveOn}
          onStart={() => {
            setIntroPlayed(true);
            setPhase("intro");
          }}
        />
      );
    }
    return <VaultView api={api} onPhase={setPhase} />;
  }
}

function CenteredMessage({
  title,
  detail,
  error,
  loading = false,
}: {
  title: MessageKey;
  detail: MessageKey;
  error?: unknown;
  loading?: boolean;
}) {
  const { t, failure } = useTranslator();
  return (
    <VaultScreen
      title={t(title)}
      description={
        <span
          className="flex max-w-[380px] items-start justify-center gap-2"
          role={loading ? "status" : "alert"}
        >
          {loading ? (
            <LoaderCircle
              className="mt-0.5 size-4 shrink-0 animate-spin motion-reduce:animate-none"
              aria-hidden="true"
            />
          ) : (
            <CircleAlert
              className="mt-0.5 size-4 shrink-0"
              aria-hidden="true"
            />
          )}
          <span>{failure(error, detail)}</span>
        </span>
      }
    />
  );
}
