import { AccessField } from "@ravenpass/ui/components/AccessField.tsx";
import { AccessShell } from "@ravenpass/ui/components/AccessShell.tsx";
import { BlockNote } from "@ravenpass/ui/components/Block.tsx";
import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import type { MessageKey } from "@ravenpass/ui/i18n/messages.ts";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { ClipboardPaste, Link } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import { toast } from "sonner";
import { ask, type LinkRefusal } from "../messages.ts";
import { ClipboardKeys } from "./clipboard.ts";

const clipboard = new ClipboardKeys();

const refusals: Record<LinkRefusal, MessageKey> = {
  "not-key": "extension.link.error.not-key",
  expired: "extension.link.error.expired",
  "not-open": "extension.link.error.not-open",
  failed: "extension.link.error.failed",
};

export function LinkScreen({ onLinked }: { onLinked: () => Promise<void> }) {
  const { t } = useTranslator();
  const [key, setKey] = useState("");
  const [busy, setBusy] = useState(false);
  const empty = key.trim() === "";

  useEffect(() => {
    let active = true;
    clipboard.granted().then(
      (found) => {
        if (active && found) setKey((typed) => typed || found);
      },
      // A clipboard that cannot be read leaves the key field as typed.
      () => {},
    );
    return () => {
      active = false;
    };
  }, []);

  async function paste() {
    try {
      const found = await clipboard.request();
      if (found) setKey(found);
      else toast.error(t("extension.link.error.not-key"));
    } catch {
      toast.error(t("extension.link.error.clipboard"));
    }
  }

  async function link(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (empty) return;
    setBusy(true);
    const answer = await ask({ kind: "link", key }).catch(
      () => ({ ok: false, reason: "failed" }) as const,
    );
    if (answer.ok) {
      void clipboard.release();
      await onLinked();
    } else {
      toast.error(t(refusals[answer.reason]));
    }
    setBusy(false);
  }

  return (
    <AccessShell
      layout="popup"
      icon={Link}
      title={t("extension.link.title")}
      description={t("extension.link.description")}
    >
      <form className="flex flex-col gap-1.5" onSubmit={link}>
        <label className="sr-only" htmlFor="connection-key">
          {t("extension.link.key")}
        </label>
        <AccessField
          id="connection-key"
          className="truncate font-mono text-xs"
          autoComplete="off"
          spellCheck={false}
          value={key}
          placeholder={t("extension.link.key")}
          disabled={busy}
          onChange={(event) => setKey(event.target.value)}
        />
        <Button
          type={empty ? "button" : "submit"}
          variant="raised"
          size="pill"
          className="mt-1 w-full"
          disabled={busy}
          onClick={empty ? () => void paste() : undefined}
        >
          {empty && <ClipboardPaste data-icon="inline-start" />}
          {empty
            ? t("extension.link.paste")
            : busy
              ? t("extension.link.busy")
              : t("extension.link.action")}
        </Button>
      </form>
      <BlockNote>{t("extension.link.note")}</BlockNote>
    </AccessShell>
  );
}
