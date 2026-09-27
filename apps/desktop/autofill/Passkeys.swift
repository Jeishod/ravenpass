import AuthenticationServices

/// The user verification a relying party asks for, WebAuthn §5.8.6.
enum Verification: String, Encodable, Sendable {
    case required
    case preferred
    case discouraged

    /// WebAuthn reads a value it does not know as preferred.
    init(_ preference: ASAuthorizationPublicKeyCredentialUserVerificationPreference) {
        switch preference {
        case .required: self = .required
        case .discouraged: self = .discouraged
        default: self = .preferred
        }
    }

    var asksOwner: Bool {
        self != .discouraged
    }
}

struct Passkey: Decodable, Sendable {
    let id: String
    let credentialID: Data
    let account: String
    let label: String

    /// Names the passkey to the page.
    var key: String {
        credentialID.base64EncodedString()
    }
}

/// The passkey sign-in the system asks the extension's list to answer.
struct PasskeyQuery: Sendable {
    let relyingParty: String
    let clientDataHash: Data
    /// Empty allows any passkey.
    let allowed: [Data]
    let verification: Verification

    init(_ parameters: ASPasskeyCredentialRequestParameters) {
        relyingParty = parameters.relyingPartyIdentifier
        clientDataHash = parameters.clientDataHash
        allowed = parameters.allowedCredentials
        verification = Verification(parameters.userVerificationPreference)
    }

    func signIn(with passkey: Passkey) -> PasskeySignIn {
        PasskeySignIn(
            relyingParty: relyingParty, clientDataHash: clientDataHash, credentialID: passkey.credentialID,
            id: passkey.id, verification: verification
        )
    }
}

/// id names the credential that holds the passkey.
struct PasskeySignIn: Sendable {
    let relyingParty: String
    let clientDataHash: Data
    let credentialID: Data
    let id: String
    let verification: Verification
}

extension PasskeySignIn {
    /// nil for a request that names no Ravenpass passkey.
    init?(_ request: ASPasskeyCredentialRequest) {
        guard let identity = request.credentialIdentity as? ASPasskeyCredentialIdentity,
              let id = identity.recordIdentifier
        else {
            return nil
        }
        self.init(
            relyingParty: identity.relyingPartyIdentifier, clientDataHash: request.clientDataHash,
            credentialID: identity.credentialID, id: id,
            verification: Verification(request.userVerificationPreference)
        )
    }
}

struct PasskeyAssertion: Sendable {
    let credentialID: Data
    let authenticatorData: Data
    let signature: Data
    let userHandle: Data
}

/// algorithms are COSE algorithm identifiers; excluded are credential IDs the account already has.
struct PasskeyRegistration: Sendable {
    let relyingParty: String
    let userHandle: Data
    let userName: String
    let clientDataHash: Data
    let algorithms: [Int]
    let excluded: [Data]
    let verification: Verification
}

extension PasskeyRegistration {
    init?(_ request: ASCredentialRequest) {
        guard let request = request as? ASPasskeyCredentialRequest,
              let identity = request.credentialIdentity as? ASPasskeyCredentialIdentity
        else {
            return nil
        }
        self.init(
            relyingParty: identity.relyingPartyIdentifier, userHandle: identity.userHandle, userName: identity.userName,
            clientDataHash: request.clientDataHash, algorithms: request.supportedAlgorithms.map(\.rawValue),
            excluded: (request.excludedCredentials ?? []).map(\.credentialID),
            verification: Verification(request.userVerificationPreference)
        )
    }
}

struct CreatedPasskey: Sendable {
    let credentialID: Data
    let attestationObject: Data
}
