import AppKit
import AuthenticationServices

/// The AutoFill extension; the app checks the match again before it releases any password, code or passkey.
@objc(CredentialProviderViewController)
final class CredentialProviderViewController: ASCredentialProviderViewController, AutofillPageDelegate {
    /// 640 points is 40rem, the page's desktop layout breakpoint.
    private static let size = NSSize(width: 640, height: 480)

    private let app = AppConnection()
    /// Serial: exchanges that may wait for the owner run one at a time, in the order asked.
    private let work = DispatchQueue(label: "com.dortanes.ravenpass.autofill.app")
    /// Icon and language reads, kept off the work queue while it waits for the owner.
    private let reads = DispatchQueue(label: "com.dortanes.ravenpass.autofill.reads")
    private var page: AutofillPage?
    /// Nil until known, and for a request answered without a list.
    private var listing: Listing?
    private var screen: SearchOpening?
    /// "open" requests that arrived before screen was known.
    private var opening: [PageRequest] = []
    /// Set once the request is completed or canceled; answers arriving later are dropped.
    private var finished = false

    override func loadView() {
        view = NSView(frame: NSRect(origin: .zero, size: Self.size))
        // The page has one dark theme.
        view.appearance = NSAppearance(named: .darkAqua)
        preferredContentSize = Self.size
    }

    override func prepareCredentialList(for serviceIdentifiers: [ASCredentialServiceIdentifier]) {
        offer(.password, for: serviceIdentifiers)
    }

    override func prepareCredentialList(
        for serviceIdentifiers: [ASCredentialServiceIdentifier],
        requestParameters: ASPasskeyCredentialRequestParameters
    ) {
        offer(.password, for: serviceIdentifiers, passkeys: PasskeyQuery(requestParameters))
    }

    override func prepareOneTimeCodeCredentialList(for serviceIdentifiers: [ASCredentialServiceIdentifier]) {
        offer(.code, for: serviceIdentifiers)
    }

    /// Answers only from an open vault in a running app; anything else asks the system to show the extension.
    override func provideCredentialWithoutUserInteraction(for credentialRequest: ASCredentialRequest) {
        if let passkeyRequest = credentialRequest as? ASPasskeyCredentialRequest {
            return signWithoutInteraction(PasskeySignIn(passkeyRequest))
        }
        guard let wanted = Release(credentialRequest) else {
            return cancel(.credentialIdentityNotFound)
        }
        let app = self.app
        run({ try app.release(wanted) }) { [weak self] released in
            guard let self else { return }
            do {
                self.complete(try released.get())
            } catch AppError.unreachable, AppError.outdated, AppError.locked {
                self.cancel(.userInteractionRequired)
            } catch AppError.notFound, AppError.noMatch, AppError.noCode {
                self.cancel(.credentialIdentityNotFound)
            } catch {
                self.cancel(.failed)
            }
        }
    }

    override func prepareInterfaceToProvideCredential(for credentialRequest: ASCredentialRequest) {
        if let passkeyRequest = credentialRequest as? ASPasskeyCredentialRequest {
            return signWithInterface(PasskeySignIn(passkeyRequest))
        }
        guard let wanted = Release(credentialRequest) else {
            return cancel(.credentialIdentityNotFound)
        }
        showPage()
        run({ try self.whenOpen { try self.app.release(wanted) } }) { [weak self] released in
            guard let self else { return }
            do {
                self.complete(try released.get())
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch AppError.notFound, AppError.noMatch, AppError.noCode {
                self.cancel(.credentialIdentityNotFound)
            } catch {
                self.page?.show(.failed(self.failure(of: error, otherwise: wanted.kind == .code ? .codeFailed : .fillFailed)))
            }
        }
    }

    /// Creates the passkey in the credential already holding the account's passkey for the relying party, or a new one.
    override func prepareInterface(forPasskeyRegistration registrationRequest: ASCredentialRequest) {
        guard let registration = PasskeyRegistration(registrationRequest) else {
            return cancel(.failed)
        }
        showPage()
        run({
            try self.whenOpen {
                if registration.verification.asksOwner {
                    self.show(.verifying)
                }
                return try self.app.create(registration)
            }
        }) { [weak self] created in
            guard let self else { return }
            do {
                self.complete(registration, try created.get())
            } catch AppError.excluded {
                self.cancel(.matchedExcludedCredential)
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch {
                self.page?.show(.failed(self.failure(of: error, otherwise: .saveFailed)))
            }
        }
    }

    /// Opens on the query's passkeys, else on credentials matching identifiers, else on the whole vault.
    private func offer(_ kind: SecretKind, for identifiers: [ASCredentialServiceIdentifier], passkeys query: PasskeyQuery? = nil) {
        let services = identifiers.compactMap(ServiceIdentifier.init)
        let codes = kind == .code
        showPage()
        run({
            try self.whenOpen {
                if let query {
                    return Found(passkeys: try self.app.passkeys(for: query), listed: nil)
                }
                return Found(passkeys: nil, listed: try self.app.search("", services: services, codes: codes))
            }
        }) { [weak self] found in
            guard let self else { return }
            do {
                let found = try found.get()
                let passkeys = found.passkeys?.passkeys ?? []
                self.listing = Listing(kind: kind, services: services, passkeyQuery: query, passkeys: passkeys)
                self.open(SearchOpening(
                    site: found.listed?.site ?? found.passkeys?.site ?? "", code: codes, passkey: query != nil,
                    scope: found.listed?.scope ?? "matches", results: found.listed?.suggestions ?? [],
                    passkeys: passkeys.map {
                        PagePasskey(key: $0.key, label: $0.label, account: $0.account, site: found.passkeys?.site ?? "")
                    }
                ))
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch {
                self.page?.show(.failed(self.failure(of: error, otherwise: .failed)))
            }
        }
    }

    private func open(_ screen: SearchOpening) {
        self.screen = screen
        for request in opening {
            page?.answer(request, with: screen)
        }
        opening.removeAll()
    }

    func page(_ page: AutofillPage, received request: PageRequest) {
        switch request.op {
        case "open":
            if let screen {
                page.answer(request, with: screen)
            } else {
                opening.append(request)
            }
        case "cancel": cancel(.userCanceled)
        case "search": search(request)
        case "fill": fill(request)
        case "sign-in": signIn(request)
        case "icon": readIcon(request)
        case "language": readLanguage(request)
        default: page.answer(request, status: "failed")
        }
    }

    func pageLost(_ page: AutofillPage) {
        cancel(.failed)
    }

    /// Lists what a search of the page finds, marking what already fills where the owner signs in.
    private func search(_ request: PageRequest) {
        guard let listing, let fields = request.fields(SearchFields.self) else {
            return answer(request, "failed")
        }
        let services = listing.services
        let codes = listing.kind == .code
        run({ try self.whenOpen { try self.app.search(fields.query, services: services, codes: codes) } }) { [weak self] found in
            guard let self else { return }
            do {
                let listed = try found.get()
                self.page?.answer(request, with: SearchResults(scope: listed.scope, results: listed.suggestions))
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch {
                self.answer(request, "failed")
            }
        }
    }

    /// Fills the picked password or code, first adding the website to it when the owner agreed.
    private func fill(_ request: PageRequest) {
        guard let listing, let fields = request.fields(FillFields.self) else {
            return answer(request, "failed")
        }
        let wanted = Release(kind: listing.kind, id: fields.id, services: listing.services)
        run({ try self.whenOpen { try self.release(wanted, adding: fields.add) } }) { [weak self] released in
            guard let self else { return }
            do {
                self.complete(try released.get())
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch {
                self.answer(request, self.fillStatus(error))
            }
        }
    }

    private func signIn(_ request: PageRequest) {
        guard let listing, let query = listing.passkeyQuery, let fields = request.fields(SignInFields.self),
              let passkey = listing.passkeys.first(where: { $0.key == fields.key })
        else {
            return answer(request, "failed")
        }
        let signIn = query.signIn(with: passkey)
        run({ try self.whenOpen { try self.sign(signIn) } }) { [weak self] signed in
            guard let self else { return }
            do {
                self.complete(signIn, try signed.get())
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch {
                self.answer(request, self.signInStatus(error))
            }
        }
    }

    private func readIcon(_ request: PageRequest) {
        guard let fields = request.fields(IconFields.self) else {
            return answer(request, "failed")
        }
        let app = self.app
        read({ try app.icon(fields.site) }) { [weak self] icon in
            guard let self else { return }
            if case .success(let icon) = icon {
                self.page?.answer(request, with: IconAnswer(image: icon.image, tint: icon.tint))
            } else {
                self.answer(request, "failed")
            }
        }
    }

    private func readLanguage(_ request: PageRequest) {
        let app = self.app
        read({ try app.language() }) { [weak self] settings in
            guard let self else { return }
            if case .success(let settings) = settings {
                self.page?.answer(request, with: LanguageAnswer(
                    languages: settings.languages, language: settings.language, chosen: settings.chosen
                ))
            } else {
                self.answer(request, "failed")
            }
        }
    }

    private func answer(_ request: PageRequest, _ status: String) {
        page?.answer(request, status: status)
    }

    /// Signs in without the extension only when the relying party does not ask to verify the owner.
    private func signWithoutInteraction(_ signIn: PasskeySignIn?) {
        guard let signIn else {
            return cancel(.credentialIdentityNotFound)
        }
        guard !signIn.verification.asksOwner else {
            return cancel(.userInteractionRequired)
        }
        let app = self.app
        run({ try app.sign(signIn) }) { [weak self] signed in
            guard let self else { return }
            do {
                self.complete(signIn, try signed.get())
            } catch AppError.unreachable, AppError.outdated, AppError.locked {
                self.cancel(.userInteractionRequired)
            } catch AppError.notFound {
                self.cancel(.credentialIdentityNotFound)
            } catch {
                self.cancel(.failed)
            }
        }
    }

    private func signWithInterface(_ signIn: PasskeySignIn?) {
        guard let signIn else {
            return cancel(.credentialIdentityNotFound)
        }
        showPage()
        run({ try self.whenOpen { try self.sign(signIn) } }) { [weak self] signed in
            guard let self else { return }
            do {
                self.complete(signIn, try signed.get())
            } catch AppError.declined {
                self.cancel(.userCanceled)
            } catch AppError.notFound {
                self.cancel(.credentialIdentityNotFound)
            } catch {
                self.page?.show(.failed(self.failure(of: error, otherwise: .signInFailed)))
            }
        }
    }

    /// Shows the page, which loads while the request's first exchange runs.
    private func showPage() {
        guard page == nil else { return }
        guard let files = Bundle.main.resourceURL?.appending(path: "autofill", directoryHint: .isDirectory) else {
            return cancel(.failed)
        }
        let shown = AutofillPage(files: files)
        shown.delegate = self
        shown.view.frame = view.bounds
        shown.view.autoresizingMask = [.width, .height]
        view.addSubview(shown.view)
        page = shown
        shown.load()
    }

    /// Runs on the work queue; with adding, the most specific website is added to the credential first.
    private nonisolated func release(_ wanted: Release, adding: Bool) throws -> Secret {
        if adding {
            do {
                try app.addSite(for: wanted)
            } catch AppError.failed {
                throw AppError.notAdded
            }
        }
        return try app.release(wanted)
    }

    /// Runs on the work queue.
    private nonisolated func sign(_ signIn: PasskeySignIn) throws -> PasskeyAssertion {
        if signIn.verification.asksOwner {
            show(.verifying)
        }
        return try app.sign(signIn)
    }

    /// Runs on the work queue; retries ask after an unlock when the vault is or becomes locked.
    private nonisolated func whenOpen<T>(_ ask: () throws -> T) throws -> T {
        if try app.status(launching: { show(.opening) }) {
            do {
                return try ask()
            } catch AppError.locked {}
        }
        show(.unlocking)
        guard try app.unlock() else {
            throw AppError.declined
        }
        return try ask()
    }

    /// Answers on the main thread with the page's wait ended, unless the request finished meanwhile.
    private func run<T: Sendable>(
        _ ask: @escaping @Sendable () throws -> T, answered: @escaping @MainActor (Result<T, Error>) -> Void
    ) {
        perform(on: work, ask) { [weak self] result in
            self?.page?.show(nil)
            answered(result)
        }
    }

    /// Answers on the main thread unless the request finished meanwhile.
    private func read<T: Sendable>(
        _ ask: @escaping @Sendable () throws -> T, answered: @escaping @MainActor (Result<T, Error>) -> Void
    ) {
        perform(on: reads, ask, answered: answered)
    }

    private func perform<T: Sendable>(
        on queue: DispatchQueue, _ ask: @escaping @Sendable () throws -> T,
        answered: @escaping @MainActor (Result<T, Error>) -> Void
    ) {
        queue.async { [weak self] in
            let result = Result(catching: ask)
            DispatchQueue.main.async {
                guard let self, !self.finished else { return }
                answered(result)
            }
        }
    }

    /// Shows wait on the page from the work queue.
    private nonisolated func show(_ wait: PageWait) {
        DispatchQueue.main.async { [weak self] in
            guard let self, !self.finished else { return }
            self.page?.show(wait)
        }
    }

    /// Why a request the page shows no list for stopped: what error names, else otherwise.
    private func failure(of error: Error, otherwise: PageFailure) -> PageFailure {
        switch error as? AppError {
        case .unreachable?: .unreachable
        case .outdated?: .outdated
        case .unverifiable?: .unverifiable
        case .unsupported?: .unsupported
        default: otherwise
        }
    }

    /// How a fill the page asked for ended, as the page reads it.
    private func fillStatus(_ error: Error) -> String {
        switch error as? AppError {
        case .noCode?: "no-code"
        case .notAdded?: "not-added"
        case .notFound?: "not-found"
        case .unreachable?: "unreachable"
        case .outdated?: "outdated"
        default: "failed"
        }
    }

    /// How a sign-in the page asked for ended, as the page reads it.
    private func signInStatus(_ error: Error) -> String {
        switch error as? AppError {
        case .notFound?: "not-found"
        case .unverifiable?: "unverifiable"
        case .unreachable?: "unreachable"
        case .outdated?: "outdated"
        default: "failed"
        }
    }

    private func complete(_ secret: Secret) {
        finished = true
        switch secret {
        case .password(let user, let password):
            extensionContext.completeRequest(withSelectedCredential: ASPasswordCredential(user: user, password: password))
        case .code(let code):
            extensionContext.completeOneTimeCodeRequest(using: ASOneTimeCodeCredential(code: code))
        }
    }

    private func complete(_ signIn: PasskeySignIn, _ assertion: PasskeyAssertion) {
        finished = true
        extensionContext.completeAssertionRequest(using: ASPasskeyAssertionCredential(
            userHandle: assertion.userHandle, relyingParty: signIn.relyingParty, signature: assertion.signature,
            clientDataHash: signIn.clientDataHash, authenticatorData: assertion.authenticatorData,
            credentialID: assertion.credentialID
        ))
    }

    private func complete(_ registration: PasskeyRegistration, _ created: CreatedPasskey) {
        finished = true
        extensionContext.completeRegistrationRequest(using: ASPasskeyRegistrationCredential(
            relyingParty: registration.relyingParty, clientDataHash: registration.clientDataHash,
            credentialID: created.credentialID, attestationObject: created.attestationObject
        ))
    }

    private func cancel(_ code: ASExtensionError.Code) {
        guard !finished else { return }
        finished = true
        app.cancel()
        extensionContext.cancelRequest(withError: ASExtensionError(code))
    }
}

/// services are most specific first.
private struct Listing {
    let kind: SecretKind
    let services: [ServiceIdentifier]
    let passkeyQuery: PasskeyQuery?
    let passkeys: [Passkey]
}

/// Exactly one of passkeys and listed is set.
private struct Found {
    let passkeys: PasskeyList?
    let listed: Listed?
}

/// A passkey request lists the relying party's passkeys alone.
private struct SearchOpening: Encodable {
    let status = "ok"
    let screen = "search"
    let site: String
    let code: Bool
    let passkey: Bool
    let scope: String
    let results: [Suggestion]
    let passkeys: [PagePasskey]
}

/// A passkey as the page lists it.
private struct PagePasskey: Encodable {
    let key: String
    let label: String
    let account: String
    let site: String
}

private struct SearchResults: Encodable {
    let status = "ok"
    let scope: String
    let results: [Suggestion]
}

private struct IconAnswer: Encodable {
    let status = "ok"
    let image: String
    let tint: String
}

private struct LanguageAnswer: Encodable {
    let status = "ok"
    let languages: [String]
    let language: String
    let chosen: Bool
}

private struct SearchFields: Decodable {
    let query: String
}

private struct FillFields: Decodable {
    let id: String
    let add: Bool
}

private struct SignInFields: Decodable {
    let key: String
}

private struct IconFields: Decodable {
    let site: String
}
