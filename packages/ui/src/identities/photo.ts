/** Stored photos and thumbnails are JPEG; a freshly cropped photo is PNG until the vault encodes it. */
const signatures: { prefix: string; type: string }[] = [
  { prefix: "/9j/", type: "image/jpeg" },
  { prefix: "iVBORw0KGgo", type: "image/png" },
];

/** A data URL for base64 JPEG or PNG bytes; null for anything else. */
export function photoSource(base64: string): string | null {
  const format = signatures.find(({ prefix }) => base64.startsWith(prefix));
  return format ? `data:${format.type};base64,${base64}` : null;
}

export interface Area {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** In whole preview pixels. */
export interface Square {
  x: number;
  y: number;
  size: number;
}

/** The host requires a whole-pixel square inside the preview. */
export function cropSquare(
  area: Area,
  preview: { width: number; height: number },
): Square {
  const size = Math.max(
    1,
    Math.min(
      Math.round(area.width),
      Math.round(area.height),
      preview.width,
      preview.height,
    ),
  );
  const x = Math.min(Math.max(Math.round(area.x), 0), preview.width - size);
  const y = Math.min(Math.max(Math.round(area.y), 0), preview.height - size);
  return { x, y, size };
}
