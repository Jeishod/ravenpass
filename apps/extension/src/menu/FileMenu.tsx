import { Avatar } from "@ravenpass/ui/components/workspace/Avatar.tsx";
import { documentTitle } from "@ravenpass/ui/components/workspace/document-types.ts";
import {
  ItemRowContent,
  itemRowClass,
} from "@ravenpass/ui/components/workspace/ItemRowContent.tsx";
import type { MessageKey } from "@ravenpass/ui/i18n/messages.ts";
import {
  type Translator,
  useTranslator,
} from "@ravenpass/ui/i18n/translator.tsx";
import { emptyDocument } from "@ravenpass/ui/identities/identity.ts";
import { SelectionGroup } from "@ravenpass/ui/motion/SelectionIndicator.tsx";
import { cn } from "cn";
import {
  Ban,
  ChevronLeft,
  FileImage,
  FileText,
  FileX,
  IdCard,
  Link2Off,
  type LucideIcon,
  ShieldAlert,
} from "lucide-react";
import { useEffect, useState } from "react";
import { accepts } from "../content/destinations.ts";
import type {
  IdentityFile,
  IdentityFiles,
  ShareProgress,
} from "../link/client.ts";
import {
  ask,
  type FileFailure,
  type FileMenuContent,
  isFrameMessage,
  type ShareFailure,
} from "../messages.ts";
import { useHighlight } from "./highlight.ts";
import {
  FailureNote,
  LockedRow,
  MenuFooter,
  NotOpenRow,
  RowChevron,
  StateRow,
  VerificationRow,
} from "./Rows.tsx";

type Listing =
  | { readonly state: "loading" }
  | { readonly state: "list"; readonly identities: readonly IdentityFiles[] }
  | { readonly state: "unavailable"; readonly reason: FileFailure };

type ShareError = Exclude<ShareFailure, "locked" | "not-open" | "unlinked">;

/** `progress` stays null until Ravenpass reports what it asks of the person. */
type Sharing =
  | { readonly state: "idle" }
  | { readonly state: "waiting"; readonly progress: ShareProgress | null }
  | { readonly state: "failed"; readonly error: ShareError };

const shareErrors: Record<
  Exclude<ShareError, "failed">,
  { icon: LucideIcon; title: MessageKey; detail: MessageKey }
> = {
  declined: {
    icon: Ban,
    title: "extension.upload.declined.title",
    detail: "extension.upload.declined.detail",
  },
  unverifiable: {
    icon: ShieldAlert,
    title: "extension.upload.unverifiable.title",
    detail: "extension.upload.unverifiable.detail",
  },
  "not-taken": {
    icon: FileX,
    title: "extension.upload.not-taken.title",
    detail: "extension.upload.not-taken.detail",
  },
};

export function FileMenu({
  token,
  site,
  content,
}: {
  token: string;
  site: string;
  content: FileMenuContent;
}) {
  const { t } = useTranslator();
  if (content.state === "no-destination") {
    return (
      <>
        <StateRow
          icon={FileX}
          title={t("extension.upload.no-destination.title")}
          detail={t("extension.upload.no-destination.detail")}
        />
        <MenuFooter />
      </>
    );
  }
  return (
    <IdentityFilesMenu token={token} site={site} accept={content.accept} />
  );
}

/** The window closes once the page takes the file. */
function IdentityFilesMenu({
  token,
  site,
  accept,
}: {
  token: string;
  site: string;
  accept: string;
}) {
  const { t } = useTranslator();
  const [listing, setListing] = useState<Listing>({ state: "loading" });
  const [chosen, setChosen] = useState<string | null>(null);
  const [sharing, setSharing] = useState<Sharing>({ state: "idle" });
  const [unlocking, setUnlocking] = useState(false);
  const [unlockFailed, setUnlockFailed] = useState(false);
  const highlight = useHighlight();

  useEffect(() => {
    let active = true;
    ask({ kind: "menu-identities", token }).then(
      (answer) => {
        if (!active) return;
        setListing(
          answer.ok
            ? { state: "list", identities: answer.identities }
            : { state: "unavailable", reason: answer.reason },
        );
      },
      () => {
        if (active) setListing({ state: "unavailable", reason: "failed" });
      },
    );
    return () => {
      active = false;
    };
  }, [token]);

  useEffect(() => {
    const onMessage = (
      message: unknown,
      sender: chrome.runtime.MessageSender,
    ): undefined => {
      if (
        sender.id === chrome.runtime.id &&
        isFrameMessage(message, token) &&
        message.kind === "share-progress"
      ) {
        const { progress } = message;
        setSharing((current) =>
          current.state === "waiting"
            ? { state: "waiting", progress }
            : current,
        );
      }
    };
    chrome.runtime.onMessage.addListener(onMessage);
    return () => chrome.runtime.onMessage.removeListener(onMessage);
  }, [token]);

  const waiting = sharing.state === "waiting";

  async function share(identity: string, file: string) {
    if (waiting) return;
    setSharing({ state: "waiting", progress: null });
    const answer = await ask({
      kind: "menu-share",
      token,
      identity,
      file,
    }).catch(() => ({ ok: false, reason: "failed" }) as const);
    if (answer.ok) return;
    const { reason } = answer;
    if (reason === "locked" || reason === "not-open" || reason === "unlinked") {
      setSharing({ state: "idle" });
      setListing({ state: "unavailable", reason });
    } else {
      setSharing({ state: "failed", error: reason });
    }
  }

  function browse(identity: string | null) {
    if (waiting) return;
    setChosen(identity);
    setSharing({ state: "idle" });
  }

  async function unlock() {
    if (unlocking) return;
    setUnlocking(true);
    setUnlockFailed(false);
    const answer = await ask({ kind: "menu-unlock", token }).catch(
      () => ({ ok: false, reason: "failed" }) as const,
    );
    setUnlocking(false);
    if (answer.ok) return;
    if (answer.reason === "failed") {
      setUnlockFailed(true);
    } else {
      setListing({ state: "unavailable", reason: answer.reason });
    }
  }

  if (listing.state === "loading") return <MenuFooter />;
  if (listing.state === "unavailable") {
    return (
      <>
        <Unavailable reason={listing.reason} onUnlock={unlock} />
        {unlockFailed && listing.reason === "locked" && (
          <FailureNote>{t("extension.menu.unlock.error")}</FailureNote>
        )}
        <MenuFooter />
      </>
    );
  }
  if (sharing.state === "waiting" && sharing.progress) {
    return (
      <>
        <VerificationRow progress={sharing.progress} subject="upload" />
        <MenuFooter />
      </>
    );
  }
  if (!listing.identities.length) {
    return (
      <>
        <StateRow
          icon={IdCard}
          title={t("extension.upload.empty.title")}
          detail={t("extension.upload.empty.detail")}
        />
        <MenuFooter />
      </>
    );
  }

  const identity = listing.identities.find(({ id }) => id === chosen);
  const rowClass = cn(
    itemRowClass(false, "menu"),
    "relative w-full disabled:opacity-50",
  );

  return (
    <>
      {sharing.state === "failed" && <ShareErrorRow error={sharing.error} />}
      <nav
        className="flex max-h-[284px] flex-col gap-1 overflow-y-auto"
        aria-label={
          identity
            ? t("extension.upload.files.label", {
                identity: identityTitle(identity, t),
              })
            : t("extension.upload.identities.label", { site })
        }
        {...highlight.list}
      >
        <SelectionGroup id="file-menu">
          {identity ? (
            <>
              <button
                type="button"
                className={rowClass}
                {...highlight.row("back")}
                onClick={() => browse(null)}
              >
                <ItemRowContent
                  active={highlight.highlighted === "back"}
                  avatar={
                    <Avatar
                      label=""
                      icon={ChevronLeft}
                      size="row"
                      shape="tile"
                      emphasis="none"
                    />
                  }
                  title={t("extension.upload.back")}
                  detail={{ text: identityTitle(identity, t) }}
                  look="menu"
                />
              </button>
              {identity.files.map((file) => {
                const key = `file:${file.id}`;
                const choosable = accepts(accept, file);
                return (
                  <button
                    key={key}
                    type="button"
                    className={rowClass}
                    disabled={!choosable}
                    {...(choosable ? highlight.row(key) : {})}
                    onClick={() => share(identity.id, file.id)}
                  >
                    <ItemRowContent
                      active={highlight.highlighted === key}
                      avatar={
                        <Avatar
                          label=""
                          photo={file.thumbnail}
                          icon={
                            file.mediaType.startsWith("image/")
                              ? FileImage
                              : FileText
                          }
                          size="row"
                          shape="tile"
                          emphasis="none"
                        />
                      }
                      title={fileTitle(file, t)}
                      detail={{
                        text: t("extension.upload.file.detail", {
                          name: file.name,
                          format: formatOf(file.mediaType),
                        }),
                      }}
                      look="menu"
                    />
                  </button>
                );
              })}
            </>
          ) : (
            listing.identities.map((entry) => {
              const key = `identity:${entry.id}`;
              const choosable = entry.files.length > 0;
              return (
                <button
                  key={key}
                  type="button"
                  className={rowClass}
                  disabled={!choosable}
                  {...(choosable ? highlight.row(key) : {})}
                  onClick={() => browse(entry.id)}
                >
                  <ItemRowContent
                    active={highlight.highlighted === key}
                    avatar={
                      <Avatar
                        label={entry.label}
                        photo={entry.thumbnail}
                        icon={IdCard}
                        size="row"
                        shape="circle"
                        emphasis="none"
                      />
                    }
                    title={identityTitle(entry, t)}
                    detail={{
                      text: choosable
                        ? holdings(entry, t)
                        : t("extension.upload.identity.empty"),
                    }}
                    trailing={
                      <RowChevron active={highlight.highlighted === key} />
                    }
                    look="menu"
                  />
                </button>
              );
            })
          )}
        </SelectionGroup>
      </nav>
      <MenuFooter actions={[identity ? "upload" : "open"]} />
    </>
  );
}

function Unavailable({
  reason,
  onUnlock,
}: {
  reason: FileFailure;
  onUnlock: () => void;
}) {
  const { t } = useTranslator();
  switch (reason) {
    case "locked":
      return (
        <LockedRow
          detail={t("extension.upload.locked.detail")}
          onUnlock={onUnlock}
        />
      );
    case "not-open":
      return <NotOpenRow detail={t("extension.upload.not-open.detail")} />;
    case "unlinked":
      return (
        <StateRow
          icon={Link2Off}
          title={t("extension.upload.unlinked.title")}
          detail={t("extension.upload.unlinked.detail")}
        />
      );
    case "failed":
      return (
        <FailureNote>{t("extension.upload.identities.error")}</FailureNote>
      );
  }
}

function ShareErrorRow({ error }: { error: ShareError }) {
  const { t } = useTranslator();
  if (error === "failed") {
    return <FailureNote>{t("extension.upload.error")}</FailureNote>;
  }
  const { icon, title, detail } = shareErrors[error];
  return (
    <div role="alert">
      <StateRow icon={icon} title={t(title)} detail={t(detail)} />
    </div>
  );
}

function identityTitle(identity: IdentityFiles, t: Translator["t"]): string {
  return identity.label || t("identity.untitled");
}

function fileTitle(file: IdentityFile, t: Translator["t"]): string {
  if (file.kind === "photo") return t("extension.upload.photo");
  return documentTitle(
    { ...emptyDocument(file.document.type), label: file.document.label },
    t,
  );
}

function holdings(identity: IdentityFiles, t: Translator["t"]): string {
  return [...new Set(identity.files.map((file) => fileTitle(file, t)))].join(
    ", ",
  );
}

function formatOf(mediaType: string): string {
  return (mediaType.split("/")[1] ?? mediaType).toUpperCase();
}
