import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ConfirmationCard } from "./components/confirmation/ConfirmationCard.tsx";
import { limitContextMenu } from "./context-menu.ts";
import { hostApi } from "./host-api.ts";
import { LanguageProvider } from "./i18n/translator.tsx";
import { MotionProvider } from "./motion/MotionProvider.tsx";
import { createQueryClient } from "./query/client.ts";
import "./styles.css";
import "./confirm.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Ravenpass could not start. Please reopen the app.");
}

limitContextMenu();

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={createQueryClient()}>
      <LanguageProvider api={hostApi}>
        <MotionProvider>
          <ConfirmationCard host={hostApi} />
        </MotionProvider>
      </LanguageProvider>
    </QueryClientProvider>
  </StrictMode>,
);
