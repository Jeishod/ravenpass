import { ask } from "../messages.ts";
import { portOffer } from "../passkeys/page-channel.ts";
import { PasskeyBridge } from "./passkey-bridge.ts";

// Mounts at document_start: the port must reach the main-world script before any page script.
export default function main(): () => void {
  const channel = new MessageChannel();
  const bridge = new PasskeyBridge({
    port: channel.port1,
    ask,
    messages: chrome.runtime.onMessage,
    extensionId: chrome.runtime.id,
  });
  bridge.start();
  postMessage(portOffer, "*", [channel.port2]);
  return () => bridge.stop();
}
