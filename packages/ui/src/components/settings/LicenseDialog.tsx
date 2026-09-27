import { useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { useTranslator } from "../../i18n/translator.tsx";
import { licenseAddress } from "../../project.ts";
import { queryKeys } from "../../query/keys.ts";
import type { AboutApi } from "../../vault-api.ts";
import {
  ownsDrag,
  ResponsiveDialog,
  ResponsiveDialogCancel,
  ResponsiveDialogContent,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";

/** LicenseDialog shows the license and third-party notices the build ships, else offers the license online. */
export function LicenseDialog({
  open,
  onOpenChange,
  about,
  onOpenWebsite,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  about: AboutApi;
  onOpenWebsite: (address: string) => void;
}) {
  const { t } = useTranslator();
  const notices = useQuery({
    queryKey: queryKeys.thirdPartyNotices,
    queryFn: () => about.thirdPartyNotices(),
    enabled: open,
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  });
  const missing = notices.isError || notices.data === "";

  return (
    <ResponsiveDialog open={open} onOpenChange={onOpenChange}>
      <ResponsiveDialogContent
        className="bg-background sm:max-w-2xl"
        showCloseButton={false}
      >
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>
            {t("settings.about.license")}
          </ResponsiveDialogTitle>
        </ResponsiveDialogHeader>
        {notices.data && (
          // Dragging over the text selects it instead of pulling the drawer closed.
          <pre
            {...ownsDrag}
            className="max-h-[60vh] min-w-0 select-text overflow-auto whitespace-pre-wrap break-words rounded-row bg-field p-3 font-mono text-[11px] leading-[1.5]"
          >
            {notices.data}
          </pre>
        )}
        {missing && (
          <p className="text-[13px] leading-[1.5] text-muted-foreground">
            {t("settings.about.license.missing")}
          </p>
        )}
        <ResponsiveDialogFooter>
          <ResponsiveDialogCancel>
            {t("settings.about.license.close")}
          </ResponsiveDialogCancel>
          {missing && (
            <Button
              type="button"
              size="pill"
              onClick={() => onOpenWebsite(licenseAddress)}
            >
              <ExternalLink data-icon="inline-start" />
              {t("settings.about.license.online")}
            </Button>
          )}
        </ResponsiveDialogFooter>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
