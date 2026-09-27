import { IdCard } from "lucide-react";
import { useTranslator } from "../../i18n/translator.tsx";
import { Validity } from "../../identities/dates.ts";
import type { IdentitySummary } from "../../vault-api.ts";
import { ItemList, type ListProps } from "./ItemList.tsx";
import type { RowDetail } from "./ItemRowContent.tsx";

export function IdentityList(props: ListProps<IdentitySummary>) {
  const { t, language } = useTranslator();
  const today = new Date();

  /** A document running out outranks the email: the row says so until it is renewed. */
  function detail(entry: IdentitySummary): RowDetail {
    const validity = Validity.of(entry.expiresOn);
    const state = validity?.state(today);
    if (validity && state === "expired") {
      return { text: t("identity.list.expired"), tone: "destructive" };
    }
    if (validity && state === "expiring") {
      return {
        text: t("identity.list.expiring", {
          relative: validity.untilExpiry(today, language),
        }),
        tone: "warning",
      };
    }
    return { text: entry.email || t("identity.email.empty") };
  }

  return (
    <ItemList
      {...props}
      label={t("identity.list.label")}
      untitled={t("identity.untitled")}
      shape="circle"
      icon={IdCard}
      detail={detail}
      picture={(entry) => entry.thumbnail}
    />
  );
}
