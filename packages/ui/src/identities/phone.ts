import type { MaskitoOptions, MaskitoPreprocessor } from "@maskito/core";
import { maskitoPhone } from "@maskito/phone";
import metadata from "libphonenumber-js/min/metadata";

/** A number starting with a digit is read as international and gets a leading `+`. */
const leadingPlus: MaskitoPreprocessor = ({ elementState, data }) => {
  const { value, selection } = elementState;
  const [from, to] = selection;
  if (/^\d/.test(value)) {
    return {
      elementState: { value: `+${value}`, selection: [from + 1, to + 1] },
      data,
    };
  }
  if (!value && /^\d/.test(data)) {
    return { elementState, data: `+${data}` };
  }
  return { elementState, data };
};

const international = maskitoPhone({ metadata, separator: " " });

/** The value is stored exactly as the mask shows it. */
export const phoneMask: MaskitoOptions = {
  ...international,
  preprocessors: [leadingPlus, ...international.preprocessors],
};
