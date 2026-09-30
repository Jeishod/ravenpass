import { useQuery } from "@tanstack/react-query";
import { IdCard } from "lucide-react";
import type { Ref } from "react";
import { toast } from "sonner";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type {
  Identity,
  IdentityInput,
  IdentitySummary,
} from "../../vault-api.ts";
import { identitySearchValues } from "../../workspace/sections.ts";
import { IdentityEditor } from "../IdentityEditor.tsx";
import { IdentityDetail } from "./IdentityDetail.tsx";
import { IdentityList } from "./IdentityList.tsx";
import {
  type CopyAction,
  type ItemKind,
  ItemPlace,
  type OpeningPane,
  type PlaceHandle,
  type PlaceMessages,
  type PlaceShell,
} from "./ItemPlace.tsx";
import type { ScanActions } from "./ScanStrip.tsx";

const messages: PlaceMessages = {
  loading: "identity.loading",
  detailLoading: "identity.detail.loading",
  selectTitle: "identity.empty.select.title",
  selectDetail: "identity.empty.select.detail",
  firstTitle: "identity.empty.first.title",
  firstDetail: "identity.empty.first.detail",
  firstAction: "identity.empty.first.action",
  emptyAll: "identity.list.empty.all",
  emptyRecent: "identity.list.empty.recent",
  emptyPinned: "identity.list.empty.pinned",
  emptySearch: "identity.list.empty.search",
  errorClose: "identity.error.close",
  errorOpen: "identity.error.open",
  errorCopy: "identity.error.copy",
  errorPin: "identity.error.pin",
  errorUnpin: "identity.error.unpin",
  errorSave: "identity.error.save",
  errorSavedPartly: "identity.error.saved-partly",
  errorDelete: "identity.error.delete",
  errorDuplicate: "identity.error.duplicate",
  errorDeletedPartly: "identity.error.deleted-partly",
};

/** IdentitiesPlace is the workspace's place for identities. */
export function IdentitiesPlace({
  shell,
  entries,
  loading,
  refresh,
  opening,
  ref,
}: {
  shell: PlaceShell;
  entries: IdentitySummary[];
  loading: boolean;
  refresh: () => Promise<void>;
  opening: OpeningPane;
  ref?: Ref<PlaceHandle>;
}) {
  const { t } = useTranslator();
  const { api, groups, busy, report } = shell;
  const { data: limits = null } = useQuery({
    queryKey: queryKeys.limits("identities"),
    queryFn: () => api.identityLimits(),
    meta: { failure: "workspace.error.read" },
  });

  function scanActions(copy: CopyAction): ScanActions {
    return {
      copy(id) {
        copy(() => api.copyScan(id), "identity.copied.scan");
      },
      async save(id) {
        try {
          if ((await api.saveScan(id)).saved) {
            toast.success(t("identity.scan.saved"));
          }
        } catch (cause) {
          report(cause, "identity.error.scan-save");
        }
      },
    };
  }

  const kind: ItemKind<Identity, IdentityInput> = {
    icon: IdCard,
    messages,
    read: (id) => api.readIdentity(id),
    create: (input, membership) => api.createIdentity(input, membership),
    update: (id, input, membership) =>
      api.updateIdentity(id, input, membership),
  };

  return (
    <ItemPlace
      ref={ref}
      shell={shell}
      kind={kind}
      entries={entries}
      loading={loading}
      refresh={refresh}
      searchValues={identitySearchValues}
      opening={opening}
      list={(props) => <IdentityList {...props} />}
      detail={(identity, controls, copy) => (
        <IdentityDetail
          identity={identity}
          controls={controls}
          scans={scanActions(copy)}
          onCopy={(field, notice) =>
            copy(() => api.copyIdentityField(identity.id, field), notice)
          }
        />
      )}
      editor={(props) => (
        <IdentityEditor
          {...props}
          groups={groups}
          limits={limits}
          busy={busy}
          host={api}
          onFailure={report}
        />
      )}
    />
  );
}
