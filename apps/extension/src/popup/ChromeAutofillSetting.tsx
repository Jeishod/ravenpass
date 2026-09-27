import { Block, BlockRow } from "@ravenpass/ui/components/Block.tsx";
import { Switch } from "@ravenpass/ui/components/ui/switch.tsx";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ChromeAutofill } from "../chrome-autofill.ts";
import { logFailure } from "../failures.ts";

const choice = new ChromeAutofill();

const readFailure = "Ravenpass could not read the Chrome autofill choice.";

/** The service worker applies the choice to Chrome once it is stored. */
export function ChromeAutofillSetting() {
  const { t } = useTranslator();
  const [off, setOff] = useState<boolean | null>(null);
  const [changing, setChanging] = useState(false);

  useEffect(() => {
    let current = true;
    // A choice that cannot be read leaves the switch disabled.
    choice.turnedOff().then(
      (value) => {
        if (current) setOff(value);
      },
      (error: unknown) => logFailure(readFailure, error),
    );
    return () => {
      current = false;
    };
  }, []);

  async function change(next: boolean) {
    setChanging(true);
    try {
      await choice.turnOff(next);
      setOff(next);
    } catch {
      toast.error(t("extension.chrome-autofill.error"));
    } finally {
      setChanging(false);
    }
  }

  return (
    <Block className="mt-3.5 text-left">
      <BlockRow
        title={t("extension.chrome-autofill")}
        detail={t("extension.chrome-autofill.detail")}
        htmlFor="chrome-autofill"
        wrap
      >
        <Switch
          id="chrome-autofill"
          checked={Boolean(off)}
          disabled={changing || off === null}
          onCheckedChange={(next) => {
            void change(next);
          }}
        />
      </BlockRow>
    </Block>
  );
}
