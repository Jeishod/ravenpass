import { KeyRound } from "lucide-react";
import { Avatar } from "./Avatar.tsx";
import { useSiteIcon } from "./SiteIcons.tsx";

/** SiteAvatar is a credential's row avatar; an empty `site` requests no icon. */
export function SiteAvatar({ site, label }: { site: string; label: string }) {
  const logo = useSiteIcon(site, site !== "");
  return (
    <Avatar
      label={label}
      logo={logo}
      icon={KeyRound}
      size="row"
      shape="tile"
      emphasis="none"
    />
  );
}
