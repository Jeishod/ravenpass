export const imports = {
  "settings.import.heading": "Import",
  "settings.import.stage.file": "Export file",
  "settings.import.stage.review": "Review",
  "settings.import.stage.result": "Result",
  "settings.import.summary":
    "Choose an export file from another password manager.",
  "settings.import.sources": "All apps",
  "settings.import.steps": "How to export",
  "settings.import.bitwarden.formats": ".json, .zip or .csv",
  "settings.import.bitwarden.step.open": "Open the export",
  "settings.import.bitwarden.step.open.detail":
    "Tools → Export in the web app, Settings → Vault options → Export in the browser extension, or Export in the desktop app.",
  "settings.import.bitwarden.step.format": "Choose the file format",
  "settings.import.bitwarden.step.format.detail":
    ".json and .zip hold every item, .csv only logins and notes. For an encrypted .json, choose Password protected.",
  "settings.import.bitwarden.review":
    "Custom fields and extra websites go into each item's notes. Favorites stay in Favorites. Password history stays in {source}.",
  "settings.import.aliasvault.formats": ".avex, .avux or .csv",
  "settings.import.aliasvault.step.open": "Open the export",
  "settings.import.aliasvault.step.open.detail":
    "Settings → Import / Export in the AliasVault web app. The mobile app exports only CSV, from Settings → Import / Export.",
  "settings.import.aliasvault.step.format": "Choose the file format",
  "settings.import.aliasvault.step.format.detail":
    "An encrypted full vault export (.avex) contains every item and requires the password you set during export. An unencrypted export (.avux) contains the same items without a password. CSV leaves out custom fields, tags, and extra one-time codes.",
  "settings.import.aliasvault.review":
    "Custom fields, alias details, extra websites and extra one-time codes go into each item's notes. Password history stays in {source}.",
  "settings.import.folders": "From {source} folders",
  "settings.import.folders-tags": "From {source} folders and tags",
  "settings.import.choose": "Choose file…",
  "settings.import.reading": "Reading…",
  "settings.import.note":
    "Ravenpass reads the file on this device and leaves it unchanged.",
  "settings.import.password.title": "Enter the file password",
  "settings.import.password.detail":
    "{name} is protected with the password you set when exporting it.",
  "settings.import.password.label": "File password",
  "settings.import.password.empty":
    "Enter the password you set when exporting.",
  "settings.import.password.cancel": "Cancel",
  "settings.import.password.continue": "Continue",
  "settings.import.password.opening": "Opening…",
  "settings.import.failed.title": "Can't import this file",
  "settings.import.failed.choose": "Choose another file…",
  "settings.import.file.detail":
    "{format} · {count, plural, one {# item} other {# items}}",
  "settings.import.format.json": "JSON file",
  "settings.import.format.encrypted-json": "Password-protected JSON",
  "settings.import.format.zip": "ZIP archive",
  "settings.import.format.encrypted-zip": "Password-protected archive",
  "settings.import.format.csv": "CSV file",
  "settings.import.change": "Change…",
  "settings.import.adding": "To be added",
  "settings.import.in-file": "{count} in the file",
  "settings.import.duplicates.skipped":
    "{count, plural, one {# is already in Ravenpass and will be skipped} other {# are already in Ravenpass and will be skipped}}",
  "settings.import.duplicates.kept":
    "{count, plural, one {# is already in Ravenpass} other {# are already in Ravenpass}}",
  "settings.import.one-time-codes":
    "{count, plural, one {# with a one-time code} other {# with one-time codes}}",
  "settings.import.from.login":
    "{count, plural, one {# from a login} other {# from logins}}",
  "settings.import.from.alias":
    "{count, plural, one {# from an alias} other {# from aliases}}",
  "settings.import.from.card":
    "{count, plural, one {# from a card} other {# from cards}}",
  "settings.import.from.identity":
    "{count, plural, one {# from an identity} other {# from identities}}",
  "settings.import.from.passport":
    "{count, plural, one {# from a passport} other {# from passports}}",
  "settings.import.from.drivers-license":
    "{count, plural, one {# from a driver's license} other {# from driver's licenses}}",
  "settings.import.from.note":
    "{count, plural, one {# from a secure note} other {# from secure notes}}",
  "settings.import.from.ssh-key":
    "{count, plural, one {# from an SSH key} other {# from SSH keys}}",
  "settings.import.from.bank-account":
    "{count, plural, one {# from a bank account} other {# from bank accounts}}",
  "settings.import.option.groups": "Turn folders into groups",
  "settings.import.option.groups.detail":
    "Each item joins the group named after its folder.",
  "settings.import.option.groups-tags": "Turn folders and tags into groups",
  "settings.import.option.groups-tags.detail":
    "Each item joins the groups named after its folder and tags.",
  "settings.import.option.skip": "Skip items already in Ravenpass",
  "settings.import.option.skip.detail":
    "Items with the same name and matching details are skipped.",
  "settings.import.left": "Not carried over",
  "settings.import.origin.login": "Login",
  "settings.import.origin.alias": "Alias",
  "settings.import.origin.card": "Card",
  "settings.import.origin.identity": "Identity",
  "settings.import.origin.passport": "Passport",
  "settings.import.origin.drivers-license": "Driver's license",
  "settings.import.origin.note": "Secure note",
  "settings.import.origin.ssh-key": "SSH key",
  "settings.import.origin.bank-account": "Bank account",
  "settings.import.origin.unknown": "Unknown item",
  "settings.import.reason.too-long": "One or more fields are too long.",
  "settings.import.reason.unnamed": "It has no name.",
  "settings.import.reason.unsupported":
    "Ravenpass doesn't support this item type.",
  "settings.import.more": "And {count} more",
  "settings.import.attachments":
    "{count, plural, one {# attached file} other {# attached files}}",
  "settings.import.attachments.detail": "They stay in the export file.",
  "settings.import.passkeys":
    "{count, plural, one {# passkey} other {# passkeys}}",
  "settings.import.passkeys.detail":
    "The logins are imported without their passkeys.",
  "settings.import.dropped":
    "{count, plural, one {# folder can't become a group} other {# folders can't become groups}}",
  "settings.import.dropped-tags":
    "{count, plural, one {# folder or tag can't become a group} other {# folders and tags can't become groups}}",
  "settings.import.dropped.detail":
    "Their names are too long, or Ravenpass holds as many groups as it can.",
  "settings.import.nothing":
    "Nothing to import: every item is already in Ravenpass or can't be imported.",
  "settings.import.cancel": "Cancel",
  "settings.import.run": "Import ({count})",
  "settings.import.importing": "Importing…",
  "settings.import.items": "{count, plural, one {# item} other {# items}}",
  "settings.import.importing.note":
    "Your vault is updated in one step after all items are ready.",
  "settings.import.done.title":
    "{count, plural, one {# item added} other {# items added}}",
  "settings.import.done.detail": "From {name}",
  "settings.import.done.groups": "New groups",
  "settings.import.warning.title": "Delete the export file",
  "settings.import.warning.detail":
    "It holds your passwords unencrypted. Move it to the Trash, then empty the Trash.",
  "settings.import.warning.zip":
    "It holds your passwords unencrypted. Save the attached files you need elsewhere, then move it to the Trash and empty the Trash.",
  "settings.import.warning.manual":
    "It holds your passwords unencrypted, so delete it from where you saved it.",
  "settings.import.warning.zip.manual":
    "It holds your passwords unencrypted. Save the attached files you need elsewhere, then delete it from where you saved it.",
  "settings.import.trash": "Move to Trash",
  "settings.import.reveal": "Show in folder",
  "settings.import.trashed":
    "The export file is in the Trash. Empty the Trash to remove it from this device.",
  "settings.import.another": "Import another file",
  "settings.import.error.choose":
    "Ravenpass couldn't read this file. Choose it again.",
  "settings.import.error.unlock":
    "Ravenpass couldn't open this file. Choose it again.",
  "settings.import.error.run":
    "Ravenpass couldn't finish the import. Your vault is unchanged. Try again.",
  "settings.import.error.refresh":
    "Your items were imported, but Ravenpass couldn't show them. Lock and reopen the vault to see them.",
  "settings.import.error.trash":
    "Ravenpass couldn't move the file to the Trash. Delete it yourself.",
  "settings.import.error.reveal":
    "Ravenpass couldn't open the folder that holds the file. Find the file and delete it.",
  "settings.import.note.website": "Website",
  "settings.import.note.one-time-code": "One-time code setup",
  "settings.import.note.title": "Title",
  "settings.import.note.company": "Company",
  "settings.import.note.username": "Username",
  "settings.import.note.expiry": "Expiration date",
  "settings.import.note.security-code": "Security code",
  "settings.import.note.card-number": "Card number",
  "settings.import.note.cardholder": "Cardholder",
  "settings.import.note.brand": "Brand",
  "settings.import.note.private-key": "Private key",
  "settings.import.note.public-key": "Public key",
  "settings.import.note.fingerprint": "Fingerprint",
  "settings.import.note.bank": "Bank",
  "settings.import.note.account-holder": "Account holder",
  "settings.import.note.account-type": "Account type",
  "settings.import.note.account-number": "Account number",
  "settings.import.note.routing-number": "Routing number",
  "settings.import.note.branch-number": "Branch number",
  "settings.import.note.pin": "PIN",
  "settings.import.note.swift": "SWIFT",
  "settings.import.note.iban": "IBAN",
  "settings.import.note.bank-phone": "Bank phone",
  "settings.import.note.license-class": "License class",
  "settings.import.note.sex": "Sex",
  "settings.import.note.birth-place": "Place of birth",
  "settings.import.note.nationality": "Nationality",
  "settings.import.note.passport-type": "Passport type",
  "settings.import.note.national-id": "National ID number",
  "settings.import.note.issued-on": "Issued on",
  "settings.import.note.expires-on": "Expires on",
  "settings.import.note.birthday": "Date of birth",
  "settings.import.note.issuer": "Issued by",
  "settings.import.note.name": "Name",
  "settings.import.note.gender": "Gender",
  "settings.import.note.nickname": "Nickname",
};
