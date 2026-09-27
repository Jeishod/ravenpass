import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App.tsx";
import { limitContextMenu } from "./context-menu.ts";
import { hostApi } from "./host-api.ts";
import { createQueryClient } from "./query/client.ts";
import "./styles.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Ravenpass could not start. Please reopen the app.");
}

limitContextMenu();

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={createQueryClient()}>
      <App api={hostApi} />
    </QueryClientProvider>
  </StrictMode>,
);
