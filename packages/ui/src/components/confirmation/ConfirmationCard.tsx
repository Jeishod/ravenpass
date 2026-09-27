import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, Fingerprint } from "lucide-react";
import {
  type FormEvent,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { pinReady, planCard } from "../../confirmation/card.ts";
import { failureCode } from "../../failures.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type { Confirmation, ConfirmationHost } from "../../vault-api.ts";
import { PinField } from "../PinField.tsx";
import { Button } from "../ui/button.tsx";

/** ConfirmationCard fills the confirmation panel with the oldest waiting request and reports its height to the host. */
export function ConfirmationCard({ host }: { host: ConfirmationHost }) {
  const [request, setRequest] = useState<Confirmation | null>(null);
  const frame = useRef<HTMLDivElement>(null);
  const shown = request?.id ?? "";

  useEffect(() => {
    const controller = new AbortController();
    host.awaitConfirmation(shown, controller.signal).then(
      (next) => {
        if (!controller.signal.aborted) setRequest(next);
      },
      // Aborted waits and unreadable requests fail here; the host expires the latter.
      () => {},
    );
    return () => controller.abort();
  }, [host, shown]);

  // The panel keeps its last height while no request waits, so the next one shows at once.
  useLayoutEffect(() => {
    const element = frame.current;
    if (!element) return;
    const observer = new ResizeObserver(() => {
      const height = Math.ceil(element.getBoundingClientRect().height);
      if (element.childElementCount > 0) {
        // A panel the host cannot resize keeps its last height.
        host.fitConfirmation(height).catch(() => {});
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [host]);

  return (
    <div ref={frame}>
      {request && (
        <RequestCard key={request.id} host={host} request={request} />
      )}
    </div>
  );
}

/** RequestCard answers one request; Escape declines it. */
function RequestCard({
  host,
  request,
}: {
  host: ConfirmationHost;
  request: Confirmation;
}) {
  const { t, failure } = useTranslator();
  const client = useQueryClient();
  const methodsKey = queryKeys.confirmationMethods(request.id);
  const methodsQuery = useQuery({
    queryKey: methodsKey,
    queryFn: () => host.unlockMethods(),
  });
  const methods = methodsQuery.data ?? null;
  const [pin, setPin] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const id = request.id;

  async function answer(action: () => Promise<void>, fallback: MessageKey) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (cause) {
      if (failureCode(cause) === "confirmation-ended") return;
      setError(failure(cause, fallback));
      setPin("");
      await client.invalidateQueries({ queryKey: methodsKey });
      setBusy(false);
    }
  }

  const declineOnEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape" || busy) return;
    event.preventDefault();
    void answer(
      () => host.declineConfirmation(id),
      "confirmation.decline-error",
    );
  });

  useEffect(() => {
    document.addEventListener("keydown", declineOnEscape);
    return () => document.removeEventListener("keydown", declineOnEscape);
  }, []);

  const plan = planCard(request, methods);
  const ready = pinReady(pin, methods);
  const shownError =
    error ||
    (methodsQuery.isError
      ? failure(methodsQuery.error, "unlock-methods.errors.unreadable")
      : "");

  function submitPin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !ready) return;
    void answer(() => host.confirmWithPin(id, pin), plan.error);
  }

  return (
    <section
      role="alertdialog"
      aria-labelledby="confirmation-title"
      aria-describedby="confirmation-description"
      className="grid gap-3 p-[18px]"
    >
      <header className="grid gap-1.5">
        <h1 id="confirmation-title" className="text-[15px]">
          {t(plan.title, plan.values)}
        </h1>
        <p
          id="confirmation-description"
          className="text-xs leading-[1.5] text-muted-foreground"
        >
          {t(plan.description, plan.values)}
        </p>
      </header>

      {plan.noMethod && (
        <div className="flex items-start gap-2.5 rounded-row bg-field px-[13px] py-[11px]">
          <CircleAlert
            className="mt-px size-4 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <p className="text-[11px] leading-[1.5] text-muted-foreground">
            <span className="block text-foreground">
              {t("unlock.none.title")}
            </span>
            {t("unlock.none.description")}
          </p>
        </div>
      )}

      {plan.pin && (
        <form
          id="confirmation-pin"
          className="flex flex-col gap-1.5"
          onSubmit={submitPin}
        >
          <PinField
            id="confirmation-pin-field"
            value={pin}
            onChange={setPin}
            maxLength={methods?.pinMaxLength}
            attemptsLeft={methods?.pinAttemptsLeft}
            disabled={busy}
          />
        </form>
      )}

      {shownError && (
        <p role="alert" className="text-[11px] text-destructive">
          {shownError}
        </p>
      )}

      {plan.biometry && (
        <Button
          type="button"
          variant={plan.pin ? "quiet" : "raised"}
          size="pill"
          className="w-full"
          autoFocus={!plan.pin}
          disabled={busy}
          onClick={() =>
            void answer(() => host.unlockFromConfirmation(id), plan.error)
          }
        >
          <Fingerprint data-icon="inline-start" />
          {busy ? t("unlock.action-busy") : t(plan.biometry)}
        </Button>
      )}

      <footer className="flex items-center justify-end gap-1.5">
        {plan.recovery && (
          <Button
            type="button"
            variant="ghost"
            size="pill-sm"
            className="mr-auto text-muted-foreground hover:text-foreground"
            disabled={busy}
            onClick={() =>
              void answer(() => host.recoverFromConfirmation(id), plan.error)
            }
          >
            {t("unlock.recovery.action")}
          </Button>
        )}
        <Button
          type="button"
          variant="quiet"
          size="pill"
          autoFocus={!plan.pin && !plan.biometry}
          disabled={busy}
          onClick={() =>
            void answer(
              () => host.declineConfirmation(id),
              "confirmation.decline-error",
            )
          }
        >
          {t("confirmation.decline")}
        </Button>
        {plan.pin && (
          <Button
            type="submit"
            form="confirmation-pin"
            variant="raised"
            size="pill"
            disabled={busy || !ready}
          >
            {busy ? t("unlock.pin.busy") : t(plan.pin.confirm)}
          </Button>
        )}
      </footer>
    </section>
  );
}
