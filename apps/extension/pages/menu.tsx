import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Menu } from "../src/menu/Menu.tsx";
import "../src/menu/styles.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Ravenpass could not show its menu. Focus the field again.");
}

createRoot(root).render(
  <StrictMode>
    <Menu token={location.hash.slice(1)} />
  </StrictMode>,
);
