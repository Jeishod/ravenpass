import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { SelectionGroup } from "@ravenpass/ui/motion/SelectionIndicator.tsx";
import type { Suggestion } from "../link/client.ts";
import { ask, type Listed, type SignInCardContent } from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";
import { CardHeader } from "./Card.tsx";
import { ConfirmFill } from "./ConfirmFill.tsx";
import {
  CredentialList,
  CredentialRow,
  type Credentials,
  FillFailure,
  useCredentials,
} from "./Credentials.tsx";
import { useHighlight } from "./highlight.ts";
import { VerificationRow } from "./Rows.tsx";

export function SignInCard({
  token,
  site,
  host,
  content,
}: {
  token: string;
  site: string;
  host: string;
  content: SignInCardContent;
}) {
  const { t } = useTranslator();
  const credentials = useCredentials(token, content.listing);
  const { content: listing } = credentials;

  const only =
    listing.state === "list" &&
    listing.purpose === "sign-in" &&
    listing.credentials.length === 1 &&
    listing.passkeys.length === 0
      ? listing.credentials[0]
      : null;

  return (
    <>
      <CardHeader
        title={t(
          content.purpose === "code"
            ? "extension.card.code.title"
            : "extension.card.sign-in.title",
          { site },
        )}
        onClose={() => {
          void sendIgnoringClosedPort(ask({ kind: "menu-close", token }));
        }}
      />
      {only ? (
        <OneAccount credential={only} host={host} credentials={credentials} />
      ) : (
        <CredentialList
          token={token}
          site={site}
          host={host}
          credentials={credentials}
        />
      )}
    </>
  );
}

function OneAccount({
  credential,
  host,
  credentials,
}: {
  credential: Listed<Suggestion>;
  host: string;
  credentials: Credentials;
}) {
  const { t } = useTranslator();
  const highlight = useHighlight();
  const signIn = () =>
    credentials.choose(credential, () => credentials.fill(credential.id));
  const account =
    credential.account || credential.label || t("credential.untitled");
  if (credentials.pending) {
    return (
      <ConfirmFill
        key={credentials.pending.id}
        credential={credentials.pending}
        host={host}
        purpose="sign-in"
        onConfirm={credentials.confirm}
        onCancel={credentials.cancel}
      />
    );
  }
  if (credentials.confirming) {
    return <VerificationRow progress={credentials.confirming} subject="fill" />;
  }
  return (
    <>
      <div {...highlight.list}>
        <SelectionGroup id="sign-in-card">
          <CredentialRow
            credential={credential}
            highlight={highlight}
            onChoose={signIn}
            trailing={null}
          />
        </SelectionGroup>
      </div>
      <Button
        type="button"
        variant="raised"
        size="pill"
        className="mt-1 w-full"
        data-menu-item
        onClick={signIn}
      >
        <span className="truncate">
          {t("extension.card.sign-in-as", { account })}
        </span>
      </Button>
      {credentials.failed && (
        <FillFailure
          content={credentials.content}
          failed={credentials.failed}
        />
      )}
    </>
  );
}
