import type { FieldDescription } from "./fields.ts";

/** A visible, editable text input without naming, with `shape` over it. */
export function field(shape: Partial<FieldDescription>): FieldDescription {
  return {
    type: "text",
    autocomplete: [],
    inputMode: "",
    maxLength: null,
    name: "",
    id: "",
    placeholder: "",
    label: "",
    precedesPassword: false,
    visible: true,
    editable: true,
    ...shape,
  };
}
