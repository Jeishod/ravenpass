import { useTranslator } from "../../i18n/translator.tsx";
import { addressParts } from "../../identities/identity.ts";
import type { Address, AddressPart } from "../../vault-api.ts";
import { Input } from "../ui/input.tsx";
import { addressPartNames } from "../workspace/AddressCard.tsx";
import { bareField, EditorRow } from "./EditorFields.tsx";

const partAutoComplete: Record<AddressPart, string> = {
  street: "street-address",
  city: "address-level2",
  region: "address-level1",
  postalCode: "postal-code",
  country: "country-name",
};

/** Character limits the vault core enforces on an address. */
export type AddressLimits = Record<AddressPart, number> & {
  addressLabel?: number;
};

/** AddressRows edits one address, its name first when it is named. */
export function AddressRows({
  id,
  address,
  named,
  limits,
  busy,
  onChange,
}: {
  id: string;
  address: Address;
  /** Whether the address carries a name of its own. */
  named: boolean;
  limits: AddressLimits | null;
  busy: boolean;
  onChange: (address: Address) => void;
}) {
  const { t } = useTranslator();
  const fields: {
    field: Exclude<keyof Address, "id">;
    label: string;
    limit: number | undefined;
    autoComplete: string;
    placeholder?: string;
  }[] = [
    ...(named
      ? [
          {
            field: "label" as const,
            label: t("identity.field.address-name"),
            limit: limits?.addressLabel,
            autoComplete: "off",
            placeholder: t("identity.field.address-name.placeholder"),
          },
        ]
      : []),
    ...addressParts.map((part) => ({
      field: part,
      label: t(addressPartNames[part]),
      limit: limits?.[part],
      autoComplete: partAutoComplete[part],
    })),
  ];

  return fields.map(({ field, label, limit, autoComplete, placeholder }) => (
    <EditorRow key={field} label={label} htmlFor={`${id}-${field}`}>
      <Input
        id={`${id}-${field}`}
        className={bareField}
        value={address[field]}
        onChange={(event) =>
          onChange({ ...address, [field]: event.target.value })
        }
        placeholder={placeholder}
        maxLength={limit}
        autoComplete={autoComplete}
        disabled={busy}
      />
    </EditorRow>
  ));
}
