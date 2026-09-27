import { cn } from "cn";
import type { ReactNode } from "react";
import {
  passkeyAccount,
  passkeyCreatedOn,
} from "../../credentials/passkeys.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { PasskeyView } from "../../vault-api.ts";
import { dividedRow, FieldBlock, fieldRow, TitledBlock } from "./Fields.tsx";

/** Passkeys lists a credential's passkeys; `action` adds a control at the end of each row. */
export function Passkeys({
  passkeys,
  action,
}: {
  passkeys: PasskeyView[];
  action?: (passkey: PasskeyView) => ReactNode;
}) {
  const { t, language } = useTranslator();

  return (
    <TitledBlock title={t("credential.passkeys")}>
      <FieldBlock>
        {passkeys.map((passkey) => {
          const account = passkeyAccount(passkey);
          const created = t("credential.passkey.created", {
            date: passkeyCreatedOn(passkey, language),
          });
          return (
            <div
              key={passkey.id}
              className={cn(
                "flex items-center gap-1",
                action && "pr-[7px]",
                dividedRow,
              )}
            >
              <span className={`${fieldRow} min-w-0 flex-1`}>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[13px]">
                    {passkey.site}
                  </span>
                  <span className="block truncate text-[11px] text-muted-foreground">
                    {account ? `${account} · ${created}` : created}
                  </span>
                </span>
              </span>
              {action?.(passkey)}
            </div>
          );
        })}
      </FieldBlock>
    </TitledBlock>
  );
}
