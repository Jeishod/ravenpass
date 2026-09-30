import { useQuery } from "@tanstack/react-query";
import { Fingerprint } from "lucide-react";
import {
  type BezierDefinition,
  useAnimate,
  useReducedMotionConfig,
} from "motion/react";
import { useEffect, useEffectEvent, useState } from "react";
import { toast } from "sonner";
import { failureCode } from "../failures.ts";
import { useCapabilities } from "../host/capabilities.tsx";
import { UnlockOnShow } from "../host/unlock-on-show.ts";
import { usePageVisible } from "../host/visibility.ts";
import type { MessageKey } from "../i18n/messages.ts";
import { useTranslator } from "../i18n/translator.tsx";
import { queryKeys } from "../query/keys.ts";
import { unlockMethodsQuery } from "../query/unlock-methods.ts";
import { useVaultStorage } from "../query/vault-storage.ts";
import { vaultLocation } from "../storage/location.ts";
import type { VaultApi, VaultState } from "../vault-api.ts";
import { ConfirmDialog } from "./ConfirmDialog.tsx";
import { ForwardButton } from "./ForwardButton.tsx";
import {
  type LockedScreen,
  lockedActions,
  lockedScreen,
} from "./locked-screen.ts";
import { PinField } from "./PinField.tsx";
import { RecoveryFlow } from "./RecoveryFlow.tsx";
import { Button } from "./ui/button.tsx";
import { vaultPart } from "./VaultArt.tsx";
import { VaultMenu } from "./VaultMenu.tsx";
import { VaultScreen } from "./VaultScreen.tsx";

const arrive: BezierDefinition = [0.22, 1, 0.36, 1];

type Way = "pin" | "device" | "changed-vault";

const headings: Record<LockedScreen, MessageKey> = {
  missing: "unlock.missing.title",
  restore: "unlock.restore.title",
  unlock: "unlock.title",
};

/** LockedView asks for the device's own unlock at most once per showing, where the host offers it. */
export function LockedView({
  api,
  unlockAtOnce = false,
  onPhase,
  onStart,
}: {
  api: VaultApi;
  /** Set where the owner just chose this vault, whose device unlock is then asked for at once. */
  unlockAtOnce?: boolean;
  onPhase: (phase: VaultState["phase"]) => void;
  /** Called once the only vault the device knows, with no way into it, has left the list. */
  onStart: () => void;
}) {
  const { t, failure } = useTranslator();
  const { unlockOnShow } = useCapabilities();
  const visible = usePageVisible();
  const [showings] = useState(() => new UnlockOnShow());
  const [recovering, setRecovering] = useState(false);
  const storageRead = useVaultStorage(
    api,
    "storage-unavailable.errors.unreadable",
  );
  const storage = storageRead.data ?? null;
  const methodsRead = useQuery(unlockMethodsQuery(api));
  const methods = methodsRead.data ?? null;
  const [pin, setPin] = useState("");
  const [rejections, setRejections] = useState(0);
  // The way in being tried; only its own control says it is busy.
  const [trying, setTrying] = useState<Way | null>(null);
  const busy = trying !== null;
  const [changingVault, setChangingVault] = useState(false);
  // Set while a vault the owner chose waits to ask for its device unlock.
  const [chosenToUnlock, setChosenToUnlock] = useState(unlockAtOnce);
  // Set while the host holds a vault another device changed at the same time, until the owner answers.
  const [diverged, setDiverged] = useState(false);
  // A failed read only skips the automatic prompt; the unlock button stays.
  const prompts = useQuery({
    queryKey: queryKeys.promptsUnlock,
    queryFn: () => api.promptsUnlock(),
    enabled: unlockOnShow && visible,
    staleTime: 0,
  });
  const promptAllowed =
    unlockOnShow && visible && !prompts.isFetching && prompts.data === true;
  const [scope, animate] = useAnimate<HTMLDivElement>();
  const reduced = useReducedMotionConfig() === true;

  async function swingOpen() {
    if (reduced) return;
    const vault = scope.current;
    await Promise.all([
      animate(
        vaultPart(vault, "handle"),
        { y: [0, -8] },
        { duration: 0.18, ease: "easeOut" },
      ),
      animate(
        vaultPart(vault, "door"),
        { rotateY: [0, -76] },
        { duration: 0.5, delay: 0.12, ease: arrive },
      ),
      animate(
        vaultPart(vault, "halo"),
        { opacity: [0.5, 1] },
        { duration: 0.5 },
      ),
    ]);
  }

  function shake() {
    if (reduced) return;
    animate(
      scope.current,
      { x: [0, -10, 10, -6, 6, -2, 0] },
      { duration: 0.42, ease: "easeOut" },
    );
  }

  // The owner closing a prompt the screen raised by itself is no failure.
  async function attempt(
    way: Way,
    open: () => Promise<void>,
    message: MessageKey,
    raisedBySelf = false,
  ) {
    setTrying(way);
    try {
      await open();
    } catch (reason) {
      const code = failureCode(reason);
      if (code === "vault-diverged") {
        setDiverged(true);
      } else if (!raisedBySelf || code !== "authentication-canceled") {
        toast.error(failure(reason, message));
        shake();
      }
      setPin("");
      if (way === "pin") setRejections((count) => count + 1);
      await Promise.all([
        methodsRead.refetch(),
        code === "vault-missing" ? storageRead.refetch() : null,
      ]);
      setTrying(null);
      return;
    }
    await swingOpen();
    onPhase("ready");
  }

  // Any device unlock answers a chosen vault's wait for one, so the screen never asks twice.
  function unlock(raisedBySelf = false) {
    setChosenToUnlock(false);
    return attempt(
      "device",
      () => api.unlock(),
      "unlock.errors.failed",
      raisedBySelf,
    );
  }

  // The host lets go of the held vault whether or not it opens.
  function adoptChangedVault() {
    return attempt(
      "changed-vault",
      async () => {
        try {
          await api.adoptChangedVault();
        } finally {
          setDiverged(false);
        }
      },
      "unlock.errors.adopt-failed",
    );
  }

  // Locking lets go of the vault the host holds for the owner's answer.
  async function declineChangedVault() {
    setDiverged(false);
    try {
      await api.lock();
    } catch (reason) {
      toast.error(failure(reason, "unlock.errors.back-failed"));
    }
  }

  const halted = busy || changingVault;
  const occupied = halted || recovering || diverged;
  const screen = lockedScreen(storage, methods);
  const actions = lockedActions(screen, methods);
  // Until the storage is read, a missing file is not yet known.
  const promptable = actions.deviceUnlock && !storageRead.isPending;

  const unlockOnShowing = useEffectEvent(() => void unlock(true));
  useEffect(() => {
    if (!unlockOnShow) return;
    if (
      showings.next({
        visible,
        deviceUnlock: promptable,
        allowed: promptAllowed,
        occupied,
      })
    ) {
      unlockOnShowing();
    }
  }, [unlockOnShow, showings, visible, promptable, promptAllowed, occupied]);

  // A vault the owner just chose asks for its device unlock once its own ways in are read.
  const settled =
    methods !== null && !methodsRead.isFetching && !storageRead.isFetching;
  const unlockChosen = useEffectEvent(() => {
    setChosenToUnlock(false);
    if (actions.deviceUnlock) void unlock(true);
  });
  useEffect(() => {
    if (chosenToUnlock && settled && !occupied) unlockChosen();
  }, [chosenToUnlock, settled, occupied]);

  function unlockWithPin(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    return attempt("pin", () => api.unlockWithPin(pin), "unlock.errors.failed");
  }

  // A switch that leaves the vault locked keeps this screen; `chosen` marks a vault the owner chose to open.
  async function changeVault(change: () => Promise<boolean>, chosen = false) {
    setChangingVault(true);
    try {
      if (await change()) {
        const phase = (await api.getState()).phase;
        await Promise.all([storageRead.refetch(), methodsRead.refetch()]);
        setPin("");
        if (phase !== "locked") onPhase(phase);
        else setChosenToUnlock(chosen);
      }
    } catch (reason) {
      toast.error(failure(reason, "unlock.errors.switch-failed"));
      await storageRead.refetch();
    } finally {
      setChangingVault(false);
    }
  }

  // Forgetting leaves the vault's file as it is.
  async function backToStart() {
    if (!storage) return;
    setChangingVault(true);
    try {
      await api.forgetVault(storage.path);
      onStart();
    } catch (reason) {
      toast.error(failure(reason, "unlock.errors.back-failed"));
      setChangingVault(false);
    }
  }

  if (recovering) {
    return (
      <RecoveryFlow
        api={api}
        onRecovered={() => onPhase("ready")}
        onCancel={() => setRecovering(false)}
      />
    );
  }

  // A device with no way into its only vault was just handed it and may put it down again.
  const leavable = screen !== "unlock" && storage?.vaults.length === 1;
  const pinReady = Boolean(
    methods?.pinSet && pin.length >= methods.pinMinLength,
  );
  const explanation =
    screen === "missing" && storage
      ? [
          t("unlock.missing.reason", { location: vaultLocation(storage) }),
          t("unlock.missing.next"),
        ]
      : screen === "restore"
        ? [t("unlock.restore.reason"), t("unlock.restore.next")]
        : null;

  return (
    <VaultScreen
      title={t(headings[screen])}
      description={
        explanation ? (
          <>
            <span className="block">{explanation[0]}</span>
            <span className="block">{explanation[1]}</span>
          </>
        ) : undefined
      }
      markRef={scope}
    >
      {storage && (
        <div className="mt-4">
          <VaultMenu
            vaults={storage.vaults}
            currentPath={storage.path}
            busy={halted}
            onSwitch={(path) =>
              changeVault(async () => {
                await api.switchVault(path);
                return true;
              }, true)
            }
            onCreate={() =>
              changeVault(async () => (await api.createVault()).changed)
            }
            onOpen={() =>
              changeVault(async () => (await api.openVault()).changed, true)
            }
          />
        </div>
      )}

      <div className="mt-8 flex w-full max-w-[300px] flex-col gap-2.5">
        {actions.pin && methods && (
          <form className="flex flex-col gap-2" onSubmit={unlockWithPin}>
            <PinField
              id="unlock-pin"
              className="h-12"
              value={pin}
              onChange={setPin}
              maxLength={methods.pinMaxLength}
              attemptsLeft={methods.pinAttemptsLeft}
              disabled={halted}
              rejections={rejections}
            />
            <ForwardButton
              type="submit"
              className="mt-1 h-11 w-full text-sm"
              disabled={halted || !pinReady}
            >
              {trying === "pin" ? t("unlock.pin.busy") : t("unlock.pin.action")}
            </ForwardButton>
          </form>
        )}

        {actions.deviceUnlock && (
          <Button
            type="button"
            variant={actions.pin ? "quiet" : "raised"}
            size="pill"
            className="h-11 w-full text-sm"
            onClick={() => unlock()}
            disabled={halted}
          >
            <Fingerprint data-icon="inline-start" />
            {trying === "device"
              ? t("unlock.action-busy")
              : actions.pin
                ? t("unlock.biometry-action")
                : t("unlock.action")}
          </Button>
        )}

        {actions.openFile && (
          <ForwardButton
            type="button"
            className="h-11 w-full text-sm"
            disabled={halted}
            onClick={() =>
              changeVault(async () => (await api.openVault()).changed)
            }
          >
            {t("unlock.missing.action")}
          </ForwardButton>
        )}

        {actions.recovery === "primary" && (
          <ForwardButton
            type="button"
            className="h-11 w-full text-sm"
            disabled={changingVault}
            onClick={() => setRecovering(true)}
          >
            {t("unlock.recovery.action")}
          </ForwardButton>
        )}
      </div>

      <div className="mt-5 flex items-center gap-1">
        {leavable && (
          <Button
            type="button"
            variant="ghost"
            size="pill-sm"
            className="text-muted-foreground hover:text-foreground max-sm:h-11"
            disabled={halted}
            onClick={backToStart}
          >
            {t("unlock.back")}
          </Button>
        )}
        {actions.recovery === "secondary" && (
          <Button
            type="button"
            variant="ghost"
            size="pill-sm"
            className="text-muted-foreground hover:text-foreground max-sm:h-11"
            disabled={halted}
            onClick={() => setRecovering(true)}
          >
            {t("unlock.recovery.action")}
          </Button>
        )}
      </div>

      <ConfirmDialog
        open={diverged}
        title={t("unlock.diverged.title")}
        detail={t("unlock.diverged.description")}
        confirm={t("unlock.diverged.confirm")}
        cancel={t("unlock.diverged.cancel")}
        busy={busy}
        onConfirm={() => void adoptChangedVault()}
        onCancel={() => {
          if (!busy) void declineChangedVault();
        }}
      />
    </VaultScreen>
  );
}
