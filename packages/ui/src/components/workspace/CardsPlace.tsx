import { useQuery } from "@tanstack/react-query";
import { CreditCard } from "lucide-react";
import type { Ref } from "react";
import { queryKeys } from "../../query/keys.ts";
import type { Card, CardInput, CardSummary } from "../../vault-api.ts";
import { cardSearchValues } from "../../workspace/sections.ts";
import { CardEditor } from "../CardEditor.tsx";
import { CardDetail } from "./CardDetail.tsx";
import { CardList } from "./CardList.tsx";
import {
  type ItemKind,
  ItemPlace,
  type OpeningPane,
  type PlaceHandle,
  type PlaceMessages,
  type PlaceShell,
} from "./ItemPlace.tsx";

const messages: PlaceMessages = {
  loading: "card.loading",
  detailLoading: "card.detail.loading",
  selectTitle: "card.empty.select.title",
  selectDetail: "card.empty.select.detail",
  firstTitle: "card.empty.first.title",
  firstDetail: "card.empty.first.detail",
  firstAction: "card.empty.first.action",
  emptyAll: "card.list.empty.all",
  emptyRecent: "card.list.empty.recent",
  emptyPinned: "card.list.empty.pinned",
  emptySearch: "card.list.empty.search",
  errorClose: "card.error.close",
  errorOpen: "card.error.open",
  errorCopy: "card.error.copy",
  errorPin: "card.error.pin",
  errorUnpin: "card.error.unpin",
  errorSave: "card.error.save",
  errorSavedPartly: "card.error.saved-partly",
  errorDelete: "card.error.delete",
  errorDeletedPartly: "card.error.deleted-partly",
};

/** CardsPlace is the workspace's place for payment cards. */
export function CardsPlace({
  shell,
  entries,
  loading,
  refresh,
  opening,
  bankLookup,
  ref,
}: {
  shell: PlaceShell;
  entries: CardSummary[];
  loading: boolean;
  refresh: () => Promise<void>;
  opening: OpeningPane;
  /** Whether the editor asks a bank's site for its name and colour. */
  bankLookup: boolean;
  ref?: Ref<PlaceHandle>;
}) {
  const { api, groups, busy, report } = shell;
  const { data: limits = null } = useQuery({
    queryKey: queryKeys.limits("cards"),
    queryFn: () => api.cardLimits(),
    meta: { failure: "workspace.error.read" },
  });

  async function openBankSite(card: Card) {
    try {
      await api.openWebsite(card.bankSite);
    } catch (cause) {
      report(cause, "workspace.error.website");
    }
  }

  const kind: ItemKind<Card, CardInput> = {
    icon: CreditCard,
    messages,
    read: (id) => api.readCard(id),
    create: (input, membership) => api.createCard(input, membership),
    update: (id, input, membership) => api.updateCard(id, input, membership),
  };

  return (
    <ItemPlace
      ref={ref}
      shell={shell}
      kind={kind}
      entries={entries}
      loading={loading}
      refresh={refresh}
      searchValues={cardSearchValues}
      opening={opening}
      list={(props) => <CardList {...props} />}
      detail={(card, controls, copy) => (
        <CardDetail
          card={card}
          controls={controls}
          onCopy={(field, notice) =>
            copy(() => api.copyCardField(card.id, field), notice)
          }
          onOpenBankSite={() => void openBankSite(card)}
        />
      )}
      editor={(props) => (
        <CardEditor
          {...props}
          groups={groups}
          limits={limits}
          busy={busy}
          bankLookup={bankLookup}
          host={api}
          onFailure={report}
        />
      )}
    />
  );
}
