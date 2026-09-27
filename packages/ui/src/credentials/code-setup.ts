import type { CodeSetup, CredentialSummary } from "../vault-api.ts";
import {
  credentialSearchValues,
  selectEntries,
} from "../workspace/sections.ts";

type SetupNames = Pick<CodeSetup, "issuer" | "account">;

function search(
  credentials: CredentialSummary[],
  query: string,
): CredentialSummary[] {
  return selectEntries(credentials, credentialSearchValues, "all", query);
}

/** Without a query, matches the setup's issuer or account; a setup naming neither offers every credential. */
export function codeSetupChoices(
  credentials: CredentialSummary[],
  setup: SetupNames,
  query: string,
): CredentialSummary[] {
  if (query.trim()) return search(credentials, query);
  const terms = [setup.issuer, setup.account].filter(Boolean);
  if (!terms.length) return search(credentials, "");
  const found = new Set(
    terms.flatMap((term) =>
      search(credentials, term).map((credential) => credential.id),
    ),
  );
  return search(
    credentials.filter((credential) => found.has(credential.id)),
    "",
  );
}

/** A credential made from the setup takes its issuer, else its account, as the required name. */
export function namesCredential(setup: SetupNames): boolean {
  return Boolean(setup.issuer || setup.account);
}
