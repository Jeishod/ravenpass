import { Check, Copy, LoaderCircle } from "lucide-react";
import { Fragment, useEffect, useEffectEvent, useMemo, useState } from "react";
import { toast } from "sonner";
import { Countdown } from "../../extensions/countdown.ts";
import { keyGroups } from "../../extensions/key-groups.ts";
import { ExtensionLinker, type LinkEvent } from "../../extensions/linker.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type {
  ExtensionLinking,
  ExtensionLinkOffer,
  LinkedExtension,
} from "../../vault-api.ts";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { StepNumber } from "../StepNumber.tsx";
import { Button } from "../ui/button.tsx";

const steps: readonly MessageKey[] = [
  "settings.extensions.dialog.step.open",
  "settings.extensions.dialog.step.paste",
  "settings.extensions.dialog.step.finish",
];

/** Milliseconds a completed link stays on screen before the dialog closes. */
const linkedPause = 1600;
/** Milliseconds the copy button reads as copied. */
const copiedPause = 2000;

/** `lifetime` is the key's time left, in seconds, when it was offered. */
type OfferState =
  | { step: "creating" }
  | {
      step: "waiting";
      offer: ExtensionLinkOffer;
      countdown: Countdown;
      lifetime: number;
    }
  | { step: "expired" };

/** LinkExtensionDialog offers a one-time connection key and closes shortly after an extension links; closing it cancels the key. */
export function LinkExtensionDialog({
  linking,
  onClose,
}: {
  linking: ExtensionLinking;
  onClose: () => void;
}) {
  const { t } = useTranslator();
  const linker = useMemo(() => new ExtensionLinker(linking), [linking]);
  const [round, setRound] = useState(0);
  const [linked, setLinked] = useState<LinkedExtension | null>(null);
  const close = useEffectEvent(onClose);

  useEffect(() => {
    if (!linked) return;
    const timer = setTimeout(() => close(), linkedPause);
    return () => clearTimeout(timer);
  }, [linked]);

  return (
    <ResponsiveDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <ResponsiveDialogContent className="sm:max-w-[380px]">
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>
            {t("settings.extensions.dialog.title")}
          </ResponsiveDialogTitle>
          {!linked && (
            <ResponsiveDialogDescription asChild>
              <ol className="mt-1 grid gap-2 text-left">
                {steps.map((step, index) => (
                  <li
                    key={step}
                    className="flex items-center gap-2.5 text-[13px] text-foreground"
                  >
                    <StepNumber value={index + 1} />
                    {t(step)}
                  </li>
                ))}
              </ol>
            </ResponsiveDialogDescription>
          )}
        </ResponsiveDialogHeader>
        {linked ? (
          <ResponsiveDialogDescription asChild>
            <div
              role="status"
              className="flex flex-col items-center gap-2.5 py-4 text-center"
            >
              <span className="flex size-10 items-center justify-center rounded-full bg-tile text-foreground">
                <Check className="size-5" aria-hidden="true" />
              </span>
              <span className="text-[13px] font-medium text-foreground">
                {t("settings.extensions.linked-notice", { name: linked.name })}
              </span>
            </div>
          </ResponsiveDialogDescription>
        ) : (
          <KeyOffer
            key={round}
            linker={linker}
            linking={linking}
            onLinked={setLinked}
            onClose={onClose}
            onRenew={() => setRound((current) => current + 1)}
          />
        )}
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}

/** KeyOffer is one key: created on mount and canceled on unmount. */
function KeyOffer({
  linker,
  linking,
  onLinked,
  onClose,
  onRenew,
}: {
  linker: ExtensionLinker;
  linking: ExtensionLinking;
  onLinked: (extension: LinkedExtension) => void;
  onClose: () => void;
  onRenew: () => void;
}) {
  const { t, failure } = useTranslator();
  const [state, setState] = useState<OfferState>({ step: "creating" });
  const [now, setNow] = useState(() => Date.now());

  const hear = useEffectEvent((event: LinkEvent) => {
    switch (event.kind) {
      case "offered": {
        const offeredAt = Date.now();
        const countdown = new Countdown(event.offer.expiresAt);
        setNow(offeredAt);
        setState({
          step: "waiting",
          offer: event.offer,
          countdown,
          lifetime: countdown.secondsLeft(offeredAt),
        });
        break;
      }
      case "linked":
        onLinked(event.extension);
        break;
      case "expired":
        setState({ step: "expired" });
        break;
      case "failed":
        toast.error(failure(event.cause, "settings.extensions.error.link"));
        onClose();
        break;
    }
  });
  useEffect(() => linker.offer((event) => hear(event)), [linker]);

  const waiting = state.step === "waiting";
  useEffect(() => {
    if (!waiting) return;
    const ticker = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(ticker);
  }, [waiting]);

  const expired =
    state.step === "expired" ||
    (state.step === "waiting" && state.countdown.expired(now));

  let body: React.ReactNode;
  if (expired) {
    body = (
      <div className="flex flex-col items-center gap-2.5 rounded-row bg-field px-[13px] py-4 text-center">
        <p className="text-[13px] text-muted-foreground">
          {t("settings.extensions.dialog.expired")}
        </p>
        <Button type="button" variant="raised" size="pill-sm" onClick={onRenew}>
          {t("settings.extensions.dialog.renew")}
        </Button>
      </div>
    );
  } else if (state.step === "waiting") {
    body = (
      <KeyField
        linking={linking}
        offer={state.offer}
        countdown={state.countdown}
        lifetime={state.lifetime}
        now={now}
      />
    );
  } else {
    body = (
      <span className="flex h-[74px] w-full items-center justify-center rounded-row bg-field">
        <LoaderCircle
          className="size-5 animate-spin text-muted-foreground motion-reduce:animate-none"
          aria-hidden="true"
        />
      </span>
    );
  }

  return (
    <>
      {body}
      <ResponsiveDialogFooter>
        <Button type="button" variant="quiet" size="pill" onClick={onClose}>
          {t("settings.extensions.dialog.cancel")}
        </Button>
      </ResponsiveDialogFooter>
    </>
  );
}

/** KeyField shows the key in groups, which copying leaves out, with its copy button and time left. */
function KeyField({
  linking,
  offer,
  countdown,
  lifetime,
  now,
}: {
  linking: ExtensionLinking;
  offer: ExtensionLinkOffer;
  countdown: Countdown;
  lifetime: number;
  now: number;
}) {
  const { t, failure } = useTranslator();
  const [copying, setCopying] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), copiedPause);
    return () => clearTimeout(timer);
  }, [copied]);

  async function copy() {
    setCopying(true);
    try {
      await linking.copyExtensionLinkKey();
      setCopied(true);
    } catch (cause) {
      toast.error(failure(cause, "settings.extensions.error.copy"));
    } finally {
      setCopying(false);
    }
  }

  const share = lifetime > 0 ? countdown.secondsLeft(now) / lifetime : 0;

  return (
    <div className="grid gap-2">
      <fieldset className="flex min-w-0 items-start gap-2 rounded-row border border-border bg-field py-1.5 pr-1.5 pl-3">
        <legend className="sr-only">
          {t("settings.extensions.dialog.key")}
        </legend>
        {/* No white space between groups, so a copied selection is the key alone. */}
        <p className="min-w-0 flex-1 select-all py-[3px] font-mono text-[13px] leading-[1.6]">
          {keyGroups(offer.key).map((group) => (
            <Fragment key={group.start}>
              {group.start > 0 && <wbr />}
              <span className="me-[0.75ch]">{group.text}</span>
            </Fragment>
          ))}
        </p>
        <Button
          type="button"
          variant="quiet"
          size="pill-sm"
          className="bg-tile font-normal hover:bg-field-hover"
          aria-label={t(
            copied
              ? "settings.extensions.dialog.copied"
              : "settings.extensions.dialog.copy.label",
          )}
          disabled={copying}
          onClick={() => void copy()}
        >
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {t(
            copied
              ? "settings.extensions.dialog.copied"
              : "settings.extensions.dialog.copy",
          )}
        </Button>
      </fieldset>
      <p
        role="timer"
        className="text-[11px] text-muted-foreground tabular-nums"
      >
        {t("settings.extensions.dialog.expires", {
          time: countdown.display(now),
        })}
      </p>
      <div
        className="h-[3px] overflow-hidden rounded-full bg-field"
        aria-hidden="true"
      >
        <div
          className="h-full rounded-full bg-muted-foreground/40 transition-[width] duration-1000 ease-linear motion-reduce:transition-none"
          style={{ width: `${share * 100}%` }}
        />
      </div>
    </div>
  );
}
