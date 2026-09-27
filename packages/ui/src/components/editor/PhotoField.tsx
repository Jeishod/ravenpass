import { IdCard, ImagePlus, Trash2, ZoomIn, ZoomOut } from "lucide-react";
import { useState } from "react";
import Cropper, { type Area, type Point } from "react-easy-crop";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  cropSquare,
  photoSource,
  type Square,
} from "../../identities/photo.ts";
import type { PhotoDraft, VaultApi } from "../../vault-api.ts";
import {
  ownsDrag,
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";
import { Slider } from "../ui/slider.tsx";
import { Avatar } from "../workspace/Avatar.tsx";

/** The host calls that stage, crop and let go of a chosen picture. */
export type PhotoService = Pick<
  VaultApi,
  "chooseIdentityPhoto" | "cropIdentityPhoto" | "discardIdentityPhoto"
>;

const maxZoom = 3;

/** PhotoField shows an identity's photo and chooses, crops or removes it; the host reads and cuts the picture. */
export function PhotoField({
  label,
  value,
  photos,
  busy,
  onChange,
  onFailure,
}: {
  /** The identity's name, which the circle takes its letter from when there is no photo. */
  label: string;
  /** In base64, empty for none. */
  value: string;
  photos: PhotoService;
  busy: boolean;
  onChange: (photo: string) => void;
  onFailure: (cause: unknown, message: MessageKey) => void;
}) {
  const { t } = useTranslator();
  const [draft, setDraft] = useState<PhotoDraft | null>(null);
  const [working, setWorking] = useState(false);

  async function choose() {
    setWorking(true);
    try {
      const chosen = await photos.chooseIdentityPhoto();
      if (chosen.chosen) setDraft(chosen);
    } catch (cause) {
      onFailure(cause, "identity.error.photo");
    } finally {
      setWorking(false);
    }
  }

  async function use(square: Square) {
    setWorking(true);
    try {
      onChange(await photos.cropIdentityPhoto(square.x, square.y, square.size));
      setDraft(null);
    } catch (cause) {
      onFailure(cause, "identity.error.photo-crop");
    } finally {
      setWorking(false);
    }
  }

  function cancel() {
    setDraft(null);
    photos.discardIdentityPhoto().catch((cause) => {
      onFailure(cause, "identity.error.photo-discard");
    });
  }

  return (
    <div className="flex shrink-0 items-center gap-3 px-1">
      <Avatar
        label={label}
        photo={value}
        icon={IdCard}
        size="header"
        shape="circle"
        emphasis="fill"
      />
      <div className="flex flex-wrap items-center gap-1.5">
        <Button
          type="button"
          variant="quiet"
          size="pill-sm"
          disabled={busy || working}
          onClick={choose}
        >
          <ImagePlus data-icon="inline-start" />
          {t("identity.photo.choose")}
        </Button>
        {value && (
          <Button
            type="button"
            variant="quiet"
            size="pill-sm"
            disabled={busy || working}
            onClick={() => onChange("")}
          >
            <Trash2 data-icon="inline-start" />
            {t("identity.photo.remove")}
          </Button>
        )}
      </div>
      {draft && (
        <CropDialog
          draft={draft}
          working={working}
          onUse={use}
          onCancel={cancel}
        />
      )}
    </div>
  );
}

/** CropDialog hands back the square under its round guide, in preview pixels. */
function CropDialog({
  draft,
  working,
  onUse,
  onCancel,
}: {
  draft: PhotoDraft;
  working: boolean;
  onUse: (square: Square) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslator();
  const [crop, setCrop] = useState<Point>({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [area, setArea] = useState<Area | null>(null);
  const source = photoSource(draft.preview);

  return (
    <ResponsiveDialog
      open
      dismissible={!working}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <ResponsiveDialogContent
        className="sm:max-w-[380px]"
        showCloseButton={false}
      >
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>
            {t("identity.photo.crop.title")}
          </ResponsiveDialogTitle>
          <ResponsiveDialogDescription>
            {t("identity.photo.crop.detail")}
          </ResponsiveDialogDescription>
        </ResponsiveDialogHeader>
        <div
          {...ownsDrag}
          className="relative h-[300px] overflow-hidden rounded-row bg-background"
        >
          {source && (
            <Cropper
              image={source}
              crop={crop}
              zoom={zoom}
              maxZoom={maxZoom}
              aspect={1}
              cropShape="round"
              showGrid={false}
              onCropChange={setCrop}
              onZoomChange={setZoom}
              onCropComplete={(_, pixels) => setArea(pixels)}
              style={{
                containerStyle: { background: "var(--background)" },
                cropAreaStyle: {
                  border: "1px solid oklch(1 0 0 / 85%)",
                  color: "oklch(0 0 0 / 62%)",
                },
              }}
            />
          )}
        </div>
        <div
          {...ownsDrag}
          className="flex items-center gap-2.5 px-1 text-muted-foreground"
        >
          <ZoomOut className="size-4 shrink-0" aria-hidden="true" />
          <Slider
            value={[zoom]}
            min={1}
            max={maxZoom}
            step={0.01}
            onValueChange={([next]) => {
              if (next !== undefined) setZoom(next);
            }}
            aria-label={t("identity.photo.zoom")}
            disabled={working}
          />
          <ZoomIn className="size-4 shrink-0" aria-hidden="true" />
        </div>
        <ResponsiveDialogFooter>
          <Button
            type="button"
            variant="quiet"
            size="pill"
            disabled={working}
            onClick={onCancel}
          >
            {t("identity.photo.cancel")}
          </Button>
          <Button
            type="button"
            variant="raised"
            size="pill"
            disabled={working || !area}
            onClick={() => {
              if (area) onUse(cropSquare(area, draft));
            }}
          >
            {t(working ? "identity.photo.using" : "identity.photo.use")}
          </Button>
        </ResponsiveDialogFooter>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
