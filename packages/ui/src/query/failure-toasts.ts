import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useEffectEvent } from "react";
import { toast } from "sonner";
import type { MessageKey } from "../i18n/messages.ts";
import { useTranslator } from "../i18n/translator.tsx";

/** FailureToasts shows a toast for every failed query or mutation whose meta names a failure message. */
export function FailureToasts(): null {
  const client = useQueryClient();
  const { failure } = useTranslator();
  const show = useEffectEvent((cause: unknown, message: MessageKey) => {
    toast.error(failure(cause, message));
  });

  useEffect(() => {
    const unwatchQueries = client.getQueryCache().subscribe((event) => {
      const message = event.query.meta?.failure;
      if (
        event.type === "updated" &&
        event.action.type === "error" &&
        message
      ) {
        show(event.action.error, message);
      }
    });
    const unwatchMutations = client.getMutationCache().subscribe((event) => {
      const message = event.mutation?.meta?.failure;
      if (
        event.type === "updated" &&
        event.action.type === "error" &&
        message
      ) {
        show(event.action.error, message);
      }
    });
    return () => {
      unwatchQueries();
      unwatchMutations();
    };
  }, [client]);
  return null;
}
