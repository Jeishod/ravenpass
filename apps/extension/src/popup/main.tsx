import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Popup } from "./Popup.tsx";
import "./styles.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Ravenpass could not start. Please reopen the extension.");
}

createRoot(root).render(
  <StrictMode>
    <Popup />
  </StrictMode>,
);
