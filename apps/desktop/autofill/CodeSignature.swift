import Foundation
import Security

/// Checks the code signature of the process at the other end of a socket.
enum CodeSignature {
    enum Peer {
        case ravenpass
        /// The app on disk was replaced after the process started, as an update replaces Ravenpass.
        case outdated
        case other
    }

    private static let appIdentifier = "com.dortanes.ravenpass"
    /// The team that signed this extension; nil for an ad-hoc build, which admits no app.
    private static let team: String? = ownTeam()

    /// The audit token names the peer's process for its lifetime; another process can reuse a process identifier.
    static func peer(of socket: Int32) -> Peer {
        guard let team else { return .other }
        let text = "anchor apple generic and certificate leaf[subject.OU] = \"\(team)\" and identifier \"\(appIdentifier)\""
        var token = audit_token_t()
        var length = socklen_t(MemoryLayout<audit_token_t>.size)
        var requirement: SecRequirement?
        guard getsockopt(socket, SOL_LOCAL, LOCAL_PEERTOKEN, &token, &length) == 0,
              Int(length) == MemoryLayout<audit_token_t>.size,
              SecRequirementCreateWithString(text as CFString, [], &requirement) == errSecSuccess, let requirement
        else {
            return .other
        }
        let attributes = [kSecGuestAttributeAudit: Data(bytes: &token, count: Int(length))] as CFDictionary
        var code: SecCode?
        var status = SecCodeCopyGuestWithAttributes(nil, attributes, [], &code)
        if status == errSecSuccess {
            status = code.map { SecCodeCheckValidity($0, [], requirement) } ?? errSecCSGuestInvalid
        }
        switch status {
        case errSecSuccess: return .ravenpass
        case errSecCSStaticCodeChanged: return .outdated
        default: return .other
        }
    }

    /// Only a well-formed Team ID enters a requirement.
    private static func ownTeam() -> String? {
        var code: SecCode?
        var staticCode: SecStaticCode?
        var information: CFDictionary?
        guard SecCodeCopySelf([], &code) == errSecSuccess, let code,
              SecCodeCopyStaticCode(code, [], &staticCode) == errSecSuccess, let staticCode,
              SecCodeCopySigningInformation(staticCode, SecCSFlags(rawValue: kSecCSSigningInformation), &information)
              == errSecSuccess,
              let team = (information as? [String: Any])?[kSecCodeInfoTeamIdentifier as String] as? String,
              team.count == 10, team.allSatisfy({ $0.isASCII && ($0.isUppercase || $0.isNumber) })
        else {
            return nil
        }
        return team
    }
}
