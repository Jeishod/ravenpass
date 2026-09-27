/** Logs only the error's name or type: an error's message may carry page or vault data. */
export function logFailure(failure: string, error: unknown): void {
  console.error(failure, error instanceof Error ? error.name : typeof error);
}

/** Resolves once `task` settles and never rejects; a rejection is logged once as `failure`. */
export async function logRejection(
  task: Promise<unknown>,
  failure: string,
): Promise<void> {
  try {
    await task;
  } catch (error) {
    logFailure(failure, error);
  }
}
