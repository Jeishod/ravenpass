import {
  type SignInStyle,
  signInStyleOf,
} from "@ravenpass/ui/extensions/sign-in-style.ts";
import type { LanguageArea } from "./language.ts";

/** Kept from each link greeting and removed on unlinking, so its presence also marks a linked extension. */
export const signInStyleKey = "desktopSignIn";
const key = signInStyleKey;

/** `chrome.storage.local` as far as the sign-in style uses it. */
export type SignInStyleArea = Pick<LanguageArea, "get" | "set" | "remove">;

/** Where the link client keeps the sign-in style the desktop app reports while linked. */
export interface SignInStyleStore {
  keepSignInStyle(style: SignInStyle): Promise<void>;
  forgetSignInStyle(): Promise<void>;
}

/** No style kept, or one this build does not know, reads as the card. */
export class StoredSignInStyle implements SignInStyleStore {
  private readonly area: SignInStyleArea;

  constructor(area: SignInStyleArea = chrome.storage.local) {
    this.area = area;
  }

  async signInStyle(): Promise<SignInStyle> {
    return signInStyleOf((await this.area.get([key]))[key]);
  }

  async keepSignInStyle(style: SignInStyle): Promise<void> {
    await this.area.set({ [key]: style });
  }

  async forgetSignInStyle(): Promise<void> {
    await this.area.remove(key);
  }
}
