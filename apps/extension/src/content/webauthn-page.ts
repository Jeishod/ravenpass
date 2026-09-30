import { takePortOffer } from "../passkeys/page-channel.ts";
import {
  type PageCreateCall,
  PageCredentials,
  type PageGetCall,
} from "../passkeys/page-credentials.ts";

// Extension.js compiles this module without its content-script wrapper: see extension.config.js.

// Captured at document_start, before any page script can replace them.
const container = CredentialsContainer.prototype;
const ownCreate = container.create;
const ownGet = container.get;
const ownConditional = PublicKeyCredential.isConditionalMediationAvailable;
const credentials = navigator.credentials;

const page = new PageCredentials({
  native: {
    create: (options) => ownCreate.call(credentials, options),
    get: (options) => ownGet.call(credentials, options),
    conditionalMediation: () => ownConditional.call(PublicKeyCredential),
  },
  prototypes: {
    credential: PublicKeyCredential.prototype,
    attestation: AuthenticatorAttestationResponse.prototype,
    assertion: AuthenticatorAssertionResponse.prototype,
  },
  exception: (message, name) => new DOMException(message, name),
});

const replacements = {
  create(options?: PageCreateCall): Promise<Credential | null> {
    return page.create(options);
  },
  get(options?: PageGetCall): Promise<Credential | null> {
    return page.get(options);
  },
  isConditionalMediationAvailable(): Promise<boolean> {
    return page.isConditionalMediationAvailable();
  },
};

container.create = replacements.create;
container.get = replacements.get;
PublicKeyCredential.isConditionalMediationAvailable =
  replacements.isConditionalMediationAvailable;

takePortOffer(window, (port) => page.connect(port));
