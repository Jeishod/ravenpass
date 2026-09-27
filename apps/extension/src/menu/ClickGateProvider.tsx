import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { ClickGate } from "./click-gate.ts";

const GateContext = createContext<ClickGate | null>(null);

export function ClickGateProvider({ children }: { children: ReactNode }) {
  const [gate] = useState(() => new ClickGate());
  useEffect(() => gate.start(document.documentElement), [gate]);
  return <GateContext value={gate}>{children}</GateContext>;
}

export function useClickGate(): ClickGate {
  const gate = useContext(GateContext);
  if (!gate) {
    throw new Error(
      "Ravenpass could not show its menu. Focus the field again.",
    );
  }
  return gate;
}
