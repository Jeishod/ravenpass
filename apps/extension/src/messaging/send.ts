import { logFailure } from "../failures.ts";

// Chrome's messaging errors when the receiving document, frame, tab or extension context is gone.
const closedPortErrors = [
  "Receiving end does not exist.",
  "The message port closed before a response was received.",
  "message channel closed before a response was received",
  "No tab with id",
  "Extension context invalidated.",
];

export const sendFailure = "Ravenpass could not deliver an extension message.";

export function isClosedPortError(error: unknown): boolean {
  return (
    error instanceof Error &&
    closedPortErrors.some((text) => error.message.includes(text))
  );
}

/** Resolves undefined when the receiver is gone or the message failed, and never rejects; only the error's name is logged. */
export async function sendIgnoringClosedPort<Answer>(
  sent: Promise<Answer>,
): Promise<Answer | undefined> {
  try {
    return await sent;
  } catch (error) {
    if (!isClosedPortError(error)) logFailure(sendFailure, error);
    return undefined;
  }
}
