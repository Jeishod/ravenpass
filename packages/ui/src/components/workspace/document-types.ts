import { BookUser, Car, FileText, IdCard, Landmark } from "lucide-react";
import type { ComponentType } from "react";
import type { MessageKey } from "../../i18n/messages.ts";
import { labelled } from "../../identities/identity.ts";
import type { DocumentType, IdentityDocument } from "../../vault-api.ts";

export const documentTypeNames: Record<DocumentType, MessageKey> = {
  passport: "identity.document.passport",
  "drivers-license": "identity.document.drivers-license",
  "id-card": "identity.document.id-card",
  "tax-number": "identity.document.tax-number",
  other: "identity.document.other",
};

export const documentTypeIcons: Record<
  DocumentType,
  ComponentType<{ className?: string }>
> = {
  passport: BookUser,
  "drivers-license": Car,
  "id-card": IdCard,
  "tax-number": Landmark,
  other: FileText,
};

/** documentTitle is the user's label for a labelled type, falling back to the type's name. */
export function documentTitle(
  document: IdentityDocument,
  t: (key: MessageKey) => string,
): string {
  const type = t(documentTypeNames[document.type]);
  return labelled(document.type) ? document.label || type : type;
}
