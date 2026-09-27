import {
  type QueryKey,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import type { MessageKey } from "../i18n/messages.ts";

export interface HostSettingOptions<T, A extends unknown[]> {
  key: QueryKey;
  read: () => Promise<T>;
  write: (...args: A) => Promise<unknown>;
  readFailure: MessageKey;
  writeFailure: MessageKey;
  /** False where the host lacks the setting: it is then never read and stays null. */
  enabled?: boolean;
}

export interface HostSetting<T, A extends unknown[]> {
  /** Null until read, where the host lacks the setting, or after a failed read. */
  value: T | null;
  /** change resolves once the setting has been read back, whether or not the host accepted it. */
  change: (...args: A) => Promise<void>;
  changing: boolean;
}

/** useHostSetting shows what the host recorded: every change, accepted or refused, is followed by a fresh read. */
export function useHostSetting<T, A extends unknown[]>({
  key,
  read,
  write,
  readFailure,
  writeFailure,
  enabled = true,
}: HostSettingOptions<T, A>): HostSetting<T, A> {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: key,
    queryFn: read,
    enabled,
    meta: { failure: readFailure },
  });
  const mutation = useMutation({
    mutationFn: (args: A) => write(...args),
    meta: { failure: writeFailure },
    onSettled: () => client.invalidateQueries({ queryKey: key }),
  });
  return {
    value: query.data ?? null,
    change: (...args) =>
      new Promise((resolve) => {
        mutation.mutate(args, { onSettled: () => resolve() });
      }),
    changing: mutation.isPending,
  };
}
