import type { SaveTarget } from "../../saving/offer.ts";
import { SiteAvatar } from "../workspace/SiteAvatar.tsx";

/** DestinationAvatar shows the site's icon, or a letter for a credential of another site. */
export function DestinationAvatar({
  offer,
  target,
}: {
  offer: { readonly site: string; readonly name: string };
  target: SaveTarget | null;
}) {
  return (
    <SiteAvatar
      site={target?.action === "add-site" ? "" : offer.site}
      label={target ? target.label : offer.name}
    />
  );
}
