import { useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import { Validity } from "../../identities/dates.ts";
import type { IdentityDocument, ScanSummary } from "../../vault-api.ts";
import { documentTitle } from "./document-types.ts";
import { SecretRow } from "./Fields.tsx";
import { type ScanMenu, ScanStrip } from "./ScanStrip.tsx";
import { ValidityHeading, type ValidityMessages } from "./ValidityHeading.tsx";

const documentValidity: ValidityMessages = {
  expires: "identity.document.expires",
  expired: "identity.document.expired",
  none: "identity.document.no-expiry",
};

/** DocumentCard owns the number's reveal, so it ends with the card. */
export function DocumentCard({
  document,
  scans,
  today,
  busy,
  onCopy,
  scanMenu,
}: {
  document: IdentityDocument;
  scans: ScanSummary[];
  today: Date;
  busy: boolean;
  onCopy: () => void;
  scanMenu: ScanMenu;
}) {
  const { t } = useTranslator();
  const [revealed, setRevealed] = useState(false);
  const title = documentTitle(document, t);

  return (
    <section
      className="shrink-0 overflow-hidden rounded-row bg-field"
      aria-label={title}
    >
      <ValidityHeading
        validity={Validity.of(document.expiresOn, document.issuedOn)}
        today={today}
        title={title}
        messages={documentValidity}
      >
        {document.issuer && (
          <span className="block truncate text-[11px] text-muted-foreground">
            {t("identity.document.issuer", { issuer: document.issuer })}
          </span>
        )}
      </ValidityHeading>
      <SecretRow
        label={t("identity.field.number")}
        value={document.number}
        revealed={revealed}
        revealLabel={t("identity.number.reveal")}
        concealLabel={t("identity.number.conceal")}
        copyLabel={t("identity.copy.number")}
        busy={busy}
        onReveal={() => setRevealed((shown) => !shown)}
        onCopy={onCopy}
      />
      {scans.length > 0 && (
        <ScanStrip
          scans={scans}
          busy={busy}
          className="px-[13px] py-2.5"
          menu={scanMenu}
        />
      )}
    </section>
  );
}
