import { LoaderCircle } from "lucide-react";
import { type FormEvent, useId, useState } from "react";
import { useTranslator } from "../i18n/translator.tsx";
import type { UnlockMethods } from "../vault-api.ts";
import { Block, BlockRow } from "./Block.tsx";
import { checkNewPin } from "./new-pin.ts";
import { PinField } from "./PinField.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "./ResponsiveDialog.tsx";
import { Button } from "./ui/button.tsx";
import { Switch } from "./ui/switch.tsx";
import { ownerCheck, type UnlockPending } from "./unlock-change.ts";

/** A change to the ways in that runs once it has the current PIN, empty where none is asked; true once saved. */
type HeldChange = (current: string) => Promise<boolean>;

/** UnlockOptions offers the ways a vault may be opened on the device, in setup and settings alike. */
export function UnlockOptions({
  methods,
  busy,
  pending = null,
  keepOne = true,
  descriptions = true,
  confirmChanges = false,
  onSetPin,
  onRemovePin,
  onBiometry,
}: {
  methods: UnlockMethods | null;
  busy: boolean;
  /** The change the device is applying; every control waits for it. */
  pending?: UnlockPending | null;
  /** Whether the last way in must stay on, as it must for a vault that exists. */
  keepOne?: boolean;
  descriptions?: boolean;
  /** Set where each change applies to the device at once, so the current PIN confirms it where the device cannot. */
  confirmChanges?: boolean;
  onSetPin: (pin: string, current: string) => Promise<boolean>;
  onRemovePin: (current: string) => Promise<boolean>;
  onBiometry: (enabled: boolean, current: string) => Promise<boolean>;
}) {
  const { t } = useTranslator();
  const [editing, setEditing] = useState(false);
  const [held, setHeld] = useState<HeldChange | null>(null);

  const busyOrUnknown = busy || pending !== null || !methods;
  const enablingBiometry = pending === "biometry-on";
  const askPin =
    confirmChanges && methods !== null && ownerCheck(methods) === "pin";
  // A PIN removed by wrong attempts leaves nothing to ask for.
  if (held !== null && !askPin) setHeld(null);
  // The last way in cannot be given up, so its switch is held where nothing would replace it.
  const biometryIsLast =
    keepOne && Boolean(methods?.biometryEnabled) && !methods?.pinSet;
  const pinIsLast =
    keepOne && Boolean(methods?.pinSet) && !methods?.biometryEnabled;

  function apply(change: HeldChange) {
    if (askPin) {
      setHeld(() => change);
      return;
    }
    void change("");
  }

  return (
    <>
      <Block>
        <BlockRow
          title={t("unlock-methods.biometry.title")}
          detail={
            enablingBiometry
              ? t("unlock-methods.biometry.creating")
              : methods && !methods.biometryAvailable
                ? t("unlock-methods.biometry.unavailable")
                : !descriptions && biometryIsLast
                  ? t("unlock-methods.biometry.last")
                  : descriptions
                    ? t("unlock-methods.biometry.description")
                    : undefined
          }
          htmlFor="unlock-biometry"
        >
          {enablingBiometry && (
            <LoaderCircle
              className="size-4 animate-spin text-muted-foreground motion-reduce:animate-none"
              aria-hidden="true"
            />
          )}
          <Switch
            id="unlock-biometry"
            checked={Boolean(methods?.biometryEnabled) || enablingBiometry}
            aria-busy={enablingBiometry || undefined}
            disabled={
              busyOrUnknown || !methods?.biometryAvailable || biometryIsLast
            }
            onCheckedChange={(enabled) =>
              apply((current) => onBiometry(enabled, current))
            }
          />
        </BlockRow>
        <BlockRow
          title={t("unlock-methods.pin.title")}
          detail={
            descriptions
              ? t("unlock-methods.pin.description", {
                  min: methods?.pinMinLength ?? 6,
                  max: methods?.pinMaxLength ?? 12,
                })
              : pinIsLast
                ? t(
                    methods?.biometryAvailable
                      ? "unlock-methods.pin.last"
                      : "unlock-methods.pin.only",
                  )
                : undefined
          }
          htmlFor="unlock-pin"
        >
          {methods?.pinSet && (
            <Button
              type="button"
              variant="quiet"
              size="pill-sm"
              disabled={busyOrUnknown}
              onClick={() => setEditing(true)}
            >
              {t("unlock-methods.pin.change")}
            </Button>
          )}
          <Switch
            id="unlock-pin"
            checked={Boolean(methods?.pinSet)}
            disabled={busyOrUnknown || pinIsLast}
            onCheckedChange={(enabled) => {
              if (enabled) {
                setEditing(true);
                return;
              }
              apply((current) => onRemovePin(current));
            }}
          />
        </BlockRow>
      </Block>

      <PinDialog
        open={editing}
        changing={Boolean(methods?.pinSet)}
        askCurrent={askPin}
        attemptsLeft={methods?.pinAttemptsLeft}
        minimum={methods?.pinMinLength ?? 6}
        maximum={methods?.pinMaxLength ?? 12}
        onSave={onSetPin}
        onClose={() => setEditing(false)}
      />
      <CurrentPinDialog
        open={held !== null}
        note={
          enablingBiometry ? t("unlock-methods.biometry.creating") : undefined
        }
        attemptsLeft={methods?.pinAttemptsLeft}
        minimum={methods?.pinMinLength ?? 6}
        maximum={methods?.pinMaxLength ?? 12}
        onConfirm={(current) => held?.(current) ?? Promise.resolve(false)}
        onClose={() => setHeld(null)}
      />
    </>
  );
}

/** PinDialog takes a new PIN twice, after the current one where the device cannot confirm the change. */
function PinDialog({
  open,
  changing,
  askCurrent,
  attemptsLeft,
  minimum,
  maximum,
  onSave,
  onClose,
}: {
  open: boolean;
  changing: boolean;
  askCurrent: boolean;
  attemptsLeft?: number;
  minimum: number;
  maximum: number;
  onSave: (pin: string, current: string) => Promise<boolean>;
  onClose: () => void;
}) {
  const { t } = useTranslator();
  const id = useId();
  const [current, setCurrent] = useState("");
  const [pin, setPin] = useState("");
  const [repeat, setRepeat] = useState("");
  const [saving, setSaving] = useState(false);

  const check = checkNewPin(pin, repeat, minimum);
  const mismatched = check === "mismatched";
  const ready =
    check === "matched" && (!askCurrent || current.length >= minimum);

  function close() {
    setCurrent("");
    setPin("");
    setRepeat("");
    onClose();
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!ready || saving) return;
    setSaving(true);
    try {
      if (await onSave(pin, askCurrent ? current : "")) close();
      else setCurrent("");
    } finally {
      setSaving(false);
    }
  }

  return (
    <ResponsiveDialog
      open={open}
      dismissible={!saving}
      onOpenChange={(next) => {
        if (!next) close();
      }}
    >
      <ResponsiveDialogContent className="bg-background sm:max-w-[360px]">
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>
            {t(
              changing ? "unlock-methods.pin.change" : "unlock-methods.pin.set",
            )}
          </ResponsiveDialogTitle>
          <ResponsiveDialogDescription>
            {t("unlock-methods.pin.weaker")}
          </ResponsiveDialogDescription>
        </ResponsiveDialogHeader>
        <form id={`${id}-form`} className="grid gap-2" onSubmit={save}>
          {askCurrent && (
            <PinField
              id={`${id}-current`}
              label={t("unlock-methods.pin.current")}
              value={current}
              onChange={setCurrent}
              maxLength={maximum}
              attemptsLeft={attemptsLeft}
              disabled={saving}
            />
          )}
          <PinField
            id={`${id}-new`}
            label={t("unlock-methods.pin.new")}
            autoFocus={!askCurrent}
            value={pin}
            onChange={setPin}
            maxLength={maximum}
            disabled={saving}
          />
          <PinField
            id={`${id}-repeat`}
            label={t("unlock-methods.pin.repeat")}
            autoFocus={false}
            invalid={mismatched}
            value={repeat}
            onChange={setRepeat}
            maxLength={maximum}
            disabled={saving}
          />
          {mismatched ? (
            <p
              role="alert"
              className="text-center text-[11px] text-destructive"
            >
              {t("unlock-methods.pin.mismatch")}
            </p>
          ) : (
            <p className="text-center text-[11px] text-muted-foreground">
              {t("unlock-methods.pin.description", {
                min: minimum,
                max: maximum,
              })}
            </p>
          )}
        </form>
        <ResponsiveDialogFooter>
          <Button
            type="button"
            variant="quiet"
            size="pill"
            disabled={saving}
            onClick={close}
          >
            {t("unlock-methods.pin.cancel")}
          </Button>
          <Button
            type="submit"
            form={`${id}-form`}
            variant="raised"
            size="pill"
            disabled={!ready || saving}
          >
            {t(
              saving ? "unlock-methods.pin.saving" : "unlock-methods.pin.save",
            )}
          </Button>
        </ResponsiveDialogFooter>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}

/** CurrentPinDialog takes the vault's PIN to confirm a change where the device cannot verify the owner. */
function CurrentPinDialog({
  open,
  note,
  attemptsLeft,
  minimum,
  maximum,
  onConfirm,
  onClose,
}: {
  open: boolean;
  /** What the change is doing while it saves. */
  note?: string;
  attemptsLeft?: number;
  minimum: number;
  maximum: number;
  onConfirm: (current: string) => Promise<boolean>;
  onClose: () => void;
}) {
  const { t } = useTranslator();
  const id = useId();
  const [current, setCurrent] = useState("");
  const [saving, setSaving] = useState(false);
  const ready = current.length >= minimum;

  function close() {
    setCurrent("");
    onClose();
  }

  async function confirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!ready || saving) return;
    setSaving(true);
    try {
      if (await onConfirm(current)) close();
      else setCurrent("");
    } finally {
      setSaving(false);
    }
  }

  return (
    <ResponsiveDialog
      open={open}
      dismissible={!saving}
      onOpenChange={(next) => {
        if (!next) close();
      }}
    >
      <ResponsiveDialogContent className="bg-background sm:max-w-[360px]">
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>
            {t("unlock-methods.confirm.title")}
          </ResponsiveDialogTitle>
          <ResponsiveDialogDescription>
            {t("unlock-methods.confirm.description")}
          </ResponsiveDialogDescription>
        </ResponsiveDialogHeader>
        <form id={`${id}-form`} className="grid gap-2" onSubmit={confirm}>
          <PinField
            id={`${id}-current`}
            value={current}
            onChange={setCurrent}
            maxLength={maximum}
            attemptsLeft={attemptsLeft}
            disabled={saving}
          />
          {saving && note && (
            <p
              role="status"
              className="text-center text-[11px] text-muted-foreground"
            >
              {note}
            </p>
          )}
        </form>
        <ResponsiveDialogFooter>
          <Button
            type="button"
            variant="quiet"
            size="pill"
            disabled={saving}
            onClick={close}
          >
            {t("unlock-methods.pin.cancel")}
          </Button>
          <Button
            type="submit"
            form={`${id}-form`}
            variant="raised"
            size="pill"
            disabled={!ready || saving}
          >
            {t(
              saving
                ? "unlock-methods.pin.saving"
                : "unlock-methods.confirm.action",
            )}
          </Button>
        </ResponsiveDialogFooter>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
