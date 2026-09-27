import type {
  Address,
  AddressPart,
  DocumentType,
  IdentityDocument,
  IdentityInput,
  ScanDraft,
  ScanMediaType,
  ScanSummary,
} from "../vault-api.ts";

/** In the order the editor offers them. */
export const documentTypes: readonly DocumentType[] = [
  "passport",
  "drivers-license",
  "id-card",
  "tax-number",
  "other",
];

export function isDocumentType(value: string): value is DocumentType {
  return (documentTypes as readonly string[]).includes(value);
}

export const emptyAddress: Address = {
  id: "",
  label: "",
  street: "",
  city: "",
  region: "",
  postalCode: "",
  country: "",
};

export function emptyDocument(type: DocumentType): IdentityDocument {
  return {
    type,
    label: "",
    number: "",
    issuer: "",
    issuedOn: "",
    expiresOn: "",
    scans: [],
  };
}

/** Scans per document. */
export const scanLimit = 4;

const scanMediaTypes: readonly ScanMediaType[] = [
  "image/jpeg",
  "application/pdf",
];

export function isScanMediaType(value: string): value is ScanMediaType {
  return (scanMediaTypes as readonly string[]).includes(value);
}

/** In the document's order; an id the identity has no scan for is skipped. */
export function scansOf(
  document: IdentityDocument,
  attachments: readonly ScanSummary[],
): ScanSummary[] {
  return document.scans.flatMap((id) => {
    const scan = attachments.find((attachment) => attachment.id === id);
    return scan ? [scan] : [];
  });
}

/** Marks a scan id as staged in the editor until a save stores it. */
const stagedPrefix = "new:";

export function stagedScan(draft: ScanDraft): ScanSummary {
  return {
    id: `${stagedPrefix}${draft.token}`,
    name: draft.name,
    mediaType: draft.mediaType,
    thumbnail: draft.thumbnail,
  };
}

/** A save deletes these if the document is gone. */
export function storedScans(document: IdentityDocument): string[] {
  return document.scans.filter((id) => !id.startsWith(stagedPrefix));
}

function blank(value: string): boolean {
  return !value.trim();
}

/** The id does not count. */
export function blankAddress(address: Address): boolean {
  const { id: _id, ...typed } = address;
  return Object.values(typed).every(blank);
}

/** Only an `other` document carries a label; a typed document is named by its type. */
export function labelled(type: DocumentType): boolean {
  return type === "other";
}

function withoutTypedLabel(document: IdentityDocument): IdentityDocument {
  return labelled(document.type) ? document : { ...document, label: "" };
}

function blankDocument(document: IdentityDocument): boolean {
  const { type: _type, scans, ...values } = document;
  return !scans.length && Object.values(values).every(blank);
}

/** Drops blank rows and typed documents' labels; other rows stay as typed so the vault can refuse them with a reason. */
export function readyToSave(input: IdentityInput): IdentityInput {
  return {
    ...input,
    emails: input.emails.filter((email) => !blank(email)),
    phones: input.phones.filter((phone) => !blank(phone)),
    addresses: input.addresses.filter((address) => !blankAddress(address)),
    documents: input.documents
      .map(withoutTypedLabel)
      .filter((document) => !blankDocument(document)),
  };
}

export function unnamedDocuments(input: IdentityInput): number {
  return readyToSave(input).documents.filter(
    (document) => labelled(document.type) && blank(document.label),
  ).length;
}

/** In reading order. */
export const addressParts: readonly AddressPart[] = [
  "street",
  "city",
  "region",
  "postalCode",
  "country",
];

/** The label, else the first filled part. */
export function addressName(address: Address): string {
  const label = address.label.trim();
  if (label) return label;
  const part = addressParts.find((name) => address[name].trim());
  return part ? address[part].trim() : "";
}
