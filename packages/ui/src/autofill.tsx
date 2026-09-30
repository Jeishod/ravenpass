import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { AutofillChannel } from "./autofill/channel.ts";
import { HostAutofill } from "./autofill/host-autofill.ts";
import { AutofillSurfaceProvider } from "./components/autofill/AutofillSheet.tsx";
import { AutofillView } from "./components/autofill/AutofillView.tsx";
import { limitContextMenu } from "./context-menu.ts";
import { showAppearance } from "./host/appearance.ts";
import { showInterfaceSize } from "./host/interface-size.ts";
import { LanguageFollower } from "./i18n/follower.ts";
import { LanguageProvider } from "./i18n/translator.tsx";
import { MotionProvider } from "./motion/MotionProvider.tsx";
import { createQueryClient } from "./query/client.ts";
import "./styles.css";
import "./autofill.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Ravenpass could not start this screen. Try again.");
}

limitContextMenu();

const { channel, surface } = AutofillChannel.ofPage();
document.documentElement.dataset.surface = surface;
const api = new HostAutofill(channel);
// Read before the first render, so no screen shows in another language, size or appearance first.
const [language, size, appearance] = await Promise.all([
  new LanguageFollower(api).current(),
  api.interfaceSize(),
  api.appearance(),
]);
showInterfaceSize(size);
showAppearance(appearance);

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={createQueryClient()}>
      <LanguageProvider api={api} initial={language}>
        <MotionProvider>
          <AutofillSurfaceProvider surface={surface}>
            <AutofillView api={api} />
          </AutofillSurfaceProvider>
        </MotionProvider>
      </LanguageProvider>
    </QueryClientProvider>
  </StrictMode>,
);
