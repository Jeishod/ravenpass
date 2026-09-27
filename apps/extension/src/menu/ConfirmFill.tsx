import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import { Checkbox } from "@ravenpass/ui/components/ui/checkbox.tsx";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { useId, useState } from "react";
import type { Suggestion, SuggestPurpose } from "../link/client.ts";
import type { Listed } from "../messages.ts";

/** Only an item saved for another site offers to remember the page; an insecure page never does. */
export function ConfirmFill({
  credential,
  host,
  purpose,
  onConfirm,
  onCancel,
}: {
  credential: Listed<Suggestion>;
  host: string;
  purpose: SuggestPurpose;
  onConfirm: (remember: boolean) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslator();
  const title = useId();
  const rememberId = useId();
  const [remember, setRemember] = useState(false);
  return (
    <section
      aria-labelledby={title}
      className="flex flex-col gap-1.5 px-2.5 pt-1.5"
    >
      <h2 id={title} className="font-medium text-[12px] text-foreground/85">
        {t(
          purpose === "code"
            ? "extension.confirm.code.title"
            : "extension.confirm.password.title",
        )}
      </h2>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className="text-muted-foreground">{t("extension.confirm.page")}</dt>
        <dd className="truncate">{host}</dd>
        <dt className="text-muted-foreground">
          {t("extension.confirm.saved")}
        </dt>
        <dd className="truncate">{credential.site}</dd>
      </dl>
      <p className="text-[11px] text-muted-foreground">
        {t(
          credential.strength === "insecure-page"
            ? "extension.confirm.insecure-page"
            : "extension.confirm.other-site",
        )}
      </p>
      {credential.strength === "other-site" && (
        <div className="flex items-center gap-2 text-[12px]">
          <Checkbox
            id={rememberId}
            checked={remember}
            onCheckedChange={(checked) => setRemember(checked === true)}
            data-menu-item
          />
          <label htmlFor={rememberId}>{t("extension.confirm.remember")}</label>
        </div>
      )}
      <div className="-mx-2.5 mt-1 flex gap-1.5">
        <Button
          type="button"
          variant="quiet"
          size="pill"
          className="flex-1"
          data-menu-item
          onClick={onCancel}
        >
          {t("extension.confirm.cancel")}
        </Button>
        <Button
          type="button"
          variant="raised"
          size="pill"
          className="flex-1"
          data-menu-item
          onClick={() => onConfirm(remember)}
        >
          {t("extension.confirm.fill")}
        </Button>
      </div>
    </section>
  );
}
