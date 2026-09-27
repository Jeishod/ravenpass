package autofill

import (
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/dortanes/ravenpass/packages/app/autofill"
)

// browserCertificates: release digests of https://www.gstatic.com/gpm-passkeys-privileged-apps/apps.json, 2026-09-24.
var browserCertificates = map[string][]string{
	"com.android.chrome": {"F0:FD:6C:5B:41:0F:25:CB:25:C3:B5:33:46:C8:97:2F:AE:30:F8:EE:74:11:DF:91:04:80:AD:6B:2D:60:DB:83"},
	"com.chrome.beta": {
		"DA:63:3D:34:B6:9E:63:AE:21:03:B4:9D:53:CE:05:2F:C5:F7:F3:C5:3A:AB:94:FD:C2:A2:08:BD:FD:14:24:9C",
		"3D:7A:12:23:01:9A:A3:9D:9E:A0:E3:43:6A:B7:C0:89:6B:FB:4F:B6:79:F4:DE:5F:E7:C2:3F:32:6C:8F:99:4A",
	},
	"com.chrome.dev": {
		"90:44:EE:5F:EE:4B:BC:5E:21:DD:44:66:54:31:C4:EB:1F:1F:71:A3:27:16:A0:BC:92:7B:CB:B3:92:33:CA:BF",
		"3D:7A:12:23:01:9A:A3:9D:9E:A0:E3:43:6A:B7:C0:89:6B:FB:4F:B6:79:F4:DE:5F:E7:C2:3F:32:6C:8F:99:4A",
	},
	"com.chrome.canary": {"20:19:DF:A1:FB:23:EF:BF:70:C5:BC:D1:44:3C:5B:EA:B0:4F:3F:2F:F4:36:6E:9A:C1:E3:45:76:39:A2:4C:FC"},
	// Firefox, Firefox Beta, Firefox Nightly and Firefox Focus.
	"org.mozilla.firefox":      {"A7:8B:62:A5:16:5B:44:94:B2:FE:AD:9E:76:A2:80:D2:2D:93:7F:EE:62:51:AE:CE:59:94:46:B2:EA:31:9B:04"},
	"org.mozilla.firefox_beta": {"A7:8B:62:A5:16:5B:44:94:B2:FE:AD:9E:76:A2:80:D2:2D:93:7F:EE:62:51:AE:CE:59:94:46:B2:EA:31:9B:04"},
	"org.mozilla.fenix":        {"50:04:77:90:88:E7:F9:88:D5:BC:5C:C5:F8:79:8F:EB:F4:F8:CD:08:4A:1B:2A:46:EF:D4:C8:EE:4A:EA:F2:11"},
	"org.mozilla.focus":        {"62:03:A4:73:BE:36:D6:4E:E3:7F:87:FA:50:0E:DB:C7:9E:AB:93:06:10:AB:9B:9F:A4:CA:7D:5C:1F:1B:4F:FC"},
	"com.brave.browser":        {"9C:2D:B7:05:13:51:5F:DB:FB:BC:58:5B:3E:DF:3D:71:23:D4:DC:67:C9:4F:FD:30:63:61:C1:D7:9B:BF:18:AC"},
	// Microsoft Edge.
	"com.microsoft.emmx":  {"01:E1:99:97:10:A8:2C:27:49:B4:D5:0C:44:5D:C8:5D:67:0B:61:36:08:9D:0A:76:6A:73:82:7C:82:A1:EA:C9"},
	"com.vivaldi.browser": {"E8:A7:85:44:65:5B:A8:C0:98:17:F7:32:76:8F:56:89:B1:66:2E:C4:B2:BC:5A:0B:C0:EC:13:8D:33:CA:3D:1E"},
	// Samsung Internet.
	"com.sec.android.app.sbrowser":  {"34:DF:0E:7A:9F:1C:F1:89:2E:45:C0:56:B4:97:3C:D8:1C:CF:14:8A:40:50:D1:1A:EA:4A:C5:A6:5F:90:0A:42"},
	"com.duckduckgo.mobile.android": {"BB:7B:B3:1C:57:3C:46:A1:DA:7F:C5:C5:28:A6:AC:F4:32:10:84:56:FE:EC:50:81:0C:7F:33:69:4E:B3:D2:D4"},
	"com.opera.browser":             {"5D:6A:FB:F8:7F:65:2A:F0:46:47:AD:A0:DF:63:4C:F2:23:70:90:0B:16:4B:09:D5:0B:D2:3A:A2:CB:52:85:B8"},
}

var browsers = func() map[string][][32]byte {
	parsed := make(map[string][][32]byte, len(browserCertificates))
	for name, certificates := range browserCertificates {
		for _, certificate := range certificates {
			digest, ok := fingerprint(certificate)
			if !ok {
				panic("autofill: malformed certificate digest of " + name)
			}
			parsed[name] = append(parsed[name], digest)
		}
	}
	return parsed
}()

// privilegedApps is the allowlist Java passes to CallingAppInfo.getOrigin, in Google's privileged-apps format.
var privilegedApps = allowlist(browserCertificates)

type privilegedApp struct {
	Type string `json:"type"`
	Info struct {
		PackageName string                `json:"package_name"`
		Signatures  []privilegedSignature `json:"signatures"`
	} `json:"info"`
}

type privilegedSignature struct {
	Build       string `json:"build"`
	Fingerprint string `json:"cert_fingerprint_sha256"`
}

func allowlist(certificates map[string][]string) string {
	apps := []privilegedApp{}
	for _, name := range slices.Sorted(maps.Keys(certificates)) {
		app := privilegedApp{Type: "android"}
		app.Info.PackageName = name
		for _, certificate := range certificates[name] {
			app.Info.Signatures = append(app.Info.Signatures, privilegedSignature{Build: "release", Fingerprint: certificate})
		}
		apps = append(apps, app)
	}
	encoded, err := json.Marshal(struct {
		Apps []privilegedApp `json:"apps"`
	}{apps})
	if err != nil {
		// Marshalling strings has no failure path in encoding/json.
		panic(err)
	}
	return string(encoded)
}

type allowlistAnswer struct {
	outcome
	Allowlist string `json:"allowlist"`
}

// trustedBrowser reports an app that is one of the browsers, signed with one of its certificates.
func trustedBrowser(app autofill.App) bool {
	certificates, listed := browsers[app.Package]
	return listed && slices.ContainsFunc(app.Signers, func(signer [32]byte) bool {
		return slices.Contains(certificates, signer)
	})
}

// fingerprint reads a hex SHA-256 digest, with or without the colons Asset Links and keytool use.
func fingerprint(text string) ([32]byte, bool) {
	var digest [32]byte
	decoded, err := hex.DecodeString(strings.ReplaceAll(text, ":", ""))
	if err != nil || len(decoded) != len(digest) {
		return digest, false
	}
	copy(digest[:], decoded)
	return digest, true
}
