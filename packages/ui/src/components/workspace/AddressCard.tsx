import { Copy } from "lucide-react";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { addressParts } from "../../identities/identity.ts";
import type { Address, AddressPart } from "../../vault-api.ts";
import { PaneAction } from "./DetailHeader.tsx";
import { FieldRow } from "./Fields.tsx";

/** What each address part is called, where it is shown and where it is typed. */
export const addressPartNames: Record<AddressPart, MessageKey> = {
  street: "identity.field.street",
  city: "identity.field.city",
  region: "identity.field.region",
  postalCode: "identity.field.postal-code",
  country: "identity.field.country",
};

const partActions: Record<AddressPart, MessageKey> = {
  street: "identity.copy.street",
  city: "identity.copy.city",
  region: "identity.copy.region",
  postalCode: "identity.copy.postal-code",
  country: "identity.copy.country",
};

const partNotices: Record<AddressPart, MessageKey> = {
  street: "identity.copied.street",
  city: "identity.copied.city",
  region: "identity.copied.region",
  postalCode: "identity.copied.postal-code",
  country: "identity.copied.country",
};

/** AddressCard copies each filled part from its own row and the whole address from the title. */
export function AddressCard({
  address,
  title,
  busy,
  onCopy,
}: {
  address: Address;
  title: string;
  busy: boolean;
  /** Copies one part, or the whole address when `part` is null. */
  onCopy: (part: AddressPart | null, notice: MessageKey) => void;
}) {
  const { t } = useTranslator();

  return (
    <section
      className="shrink-0 overflow-hidden rounded-row bg-field"
      aria-label={title}
    >
      <div className="flex min-h-[41px] items-center gap-2.5 border-b py-1.5 pr-[7px] pl-[13px]">
        <span className="min-w-0 flex-1 truncate text-[13px]">{title}</span>
        <PaneAction
          label={t("identity.copy.address")}
          icon={Copy}
          disabled={busy}
          onClick={() => onCopy(null, "identity.copied.address")}
        />
      </div>
      {addressParts
        .filter((part) => address[part].trim())
        .map((part) => (
          <FieldRow
            key={part}
            label={t(addressPartNames[part])}
            action={t(partActions[part])}
            icon={Copy}
            disabled={busy}
            onAction={() => onCopy(part, partNotices[part])}
          >
            <span className="min-w-0 flex-1 truncate text-[13px]">
              {address[part]}
            </span>
          </FieldRow>
        ))}
    </section>
  );
}
