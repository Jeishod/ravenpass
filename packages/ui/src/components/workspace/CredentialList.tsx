import { KeyRound, Timer, UserRoundKey } from "lucide-react";
import { accountOf } from "../../credentials/credential.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { CredentialSummary } from "../../vault-api.ts";
import { ItemList, type ListProps } from "./ItemList.tsx";

const markClass = "size-3.5 text-faint";

export function CredentialList(props: ListProps<CredentialSummary>) {
  const { t } = useTranslator();

  return (
    <ItemList
      {...props}
      label={t("workspace.list.label")}
      untitled={t("credential.untitled")}
      shape="tile"
      icon={KeyRound}
      detail={(entry) => ({
        text: accountOf(entry) || t("credential.login.empty"),
      })}
      site={(entry) => entry.site}
      marks={(entry) =>
        (entry.oneTimeCode || entry.passkeys > 0) && (
          <>
            {entry.oneTimeCode && (
              <Timer
                className={markClass}
                aria-label={t("credential.mark.totp")}
              />
            )}
            {entry.passkeys > 0 && (
              <UserRoundKey
                className={markClass}
                aria-label={t("credential.mark.passkey")}
              />
            )}
          </>
        )
      }
    />
  );
}
