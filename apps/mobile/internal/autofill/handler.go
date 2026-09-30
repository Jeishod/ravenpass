package autofill

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/captures"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/siteicons"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

// lookupWait bounds the Asset Links lookup within the 5 seconds Android gives a whole fill answer.
const lookupWait = 1500 * time.Millisecond

// followWait bounds the wait for the vault file within the same deadline, beside lookupWait; a slower read goes on
// after the request is answered from the vault as it is open.
const followWait = 1500 * time.Millisecond

// maxIconBytes is a siteicons.IconSize square's raw RGBA size; a larger PNG is left out of an answer.
const (
	maxIconBytes       = siteicons.IconSize * siteicons.IconSize * 4
	maxAnswerIconBytes = 256 << 10
)

// waitingSender is the one sender of the sign-ins that wait for the save screen.
const waitingSender = "system"

// Handler answers Java's JSON requests, dispatched on their "op", with JSON answers headed by a "status".
type Handler struct {
	service   Service
	vault     Vault
	owner     Owner
	icons     Icons
	page      Page
	links     *AssetLinks
	reason    func() string
	words     func(confirmation.Reason) string
	opened    func()
	follow    func()
	requested func()
	screens   screens
	waiting   *captures.Held[pendingSave]

	followMu sync.Mutex
	// following closes once the file read under way ends; nil while none runs.
	following chan struct{}
}

// Options are the dependencies of a Handler.
type Options struct {
	Service Service
	Vault   Vault
	Owner   Owner
	Icons   Icons
	Page    Page
	Links   *AssetLinks
	// Reason and Words word the unlock and passkey owner prompts in the owner's language.
	Reason func() string
	Words  func(confirmation.Reason) string
	Opened func()
	// Follow reloads an open vault from its file, which another device may have saved.
	Follow    func()
	Requested func()
	// Hold pauses the automatic lock until the returned release runs.
	Hold func() (release func())
}

// NewHandler returns a Handler over options.
func NewHandler(options Options) *Handler {
	return &Handler{
		service: options.Service, vault: options.Vault, owner: options.Owner, icons: options.Icons, page: options.Page,
		links: options.Links, reason: options.Reason, words: options.Words, opened: options.Opened, follow: options.Follow,
		requested: options.Requested, screens: screens{hold: options.Hold},
		waiting: captures.New[pendingSave](captures.Lifetime),
	}
}

// opOf is the op a request names, false for malformed JSON.
func opOf(request []byte) (string, bool) {
	var head struct {
		Op string `json:"op"`
	}
	if err := json.Unmarshal(request, &head); err != nil {
		return "", false
	}
	return head.Op, true
}

// ScreenNotice reports whether request is a screen's shown or hidden notice, which Java sends from the main thread, and
// which of the two it is.
func ScreenNotice(request []byte) (shown, notice bool) {
	op, _ := opOf(request)
	return op == "shown", op == "shown" || op == "hidden"
}

// ShowScreens holds the automatic lock for count screens shown before the handler existed.
func (h *Handler) ShowScreens(count int) {
	for range count {
		h.screens.show()
	}
}

// Noted answers a screen notice.
func Noted() []byte { return encode(outcome{Status: statusOK}) }

// Call answers one JSON request by its op; malformed JSON or an unknown op answers a failed status.
func (h *Handler) Call(request []byte) []byte {
	op, ok := opOf(request)
	if !ok {
		return encode(outcome{Status: statusFailed})
	}
	defer h.requested()
	switch op {
	case "suggest":
		h.followWithin()
		return serve(request, h.suggest)
	case "fill":
		return serve(request, h.fill)
	case "code":
		return serve(request, h.code)
	case "search":
		return serve(request, h.search)
	case "link":
		return serve(request, h.link)
	case "capture":
		h.followWithin()
		return serve(request, h.capture)
	case "offer":
		return serve(request, h.offerWaiting)
	case "save":
		return serve(request, h.save)
	case "passkeys":
		h.followWithin()
		return serve(request, h.passkeys)
	case "sign-passkey":
		return serve(request, h.signPasskey)
	case "create-passkey":
		h.followWithin()
		return serve(request, h.createPasskey)
	case "privileged-apps":
		return encode(allowlistAnswer{outcome: outcome{Status: statusOK}, Allowlist: privilegedApps})
	case "methods":
		return encode(h.methods())
	case "unlock":
		return serve(request, h.unlock)
	case "icon":
		return serve(request, h.icon)
	case "language":
		return encode(h.language())
	case "interface-size":
		return encode(h.interfaceSize())
	case "appearance":
		return encode(h.appearance())
	case "shown":
		h.screens.show()
		return Noted()
	case "hidden":
		h.screens.hide()
		return Noted()
	default:
		return encode(outcome{Status: statusFailed})
	}
}

// followWithin brings the open vault up to its file, waiting at most followWait for the read; a request made while a
// read runs waits on that one rather than starting another.
func (h *Handler) followWithin() {
	h.followMu.Lock()
	followed := h.following
	if followed == nil {
		followed = make(chan struct{})
		h.following = followed
		go func() {
			h.follow()
			h.followMu.Lock()
			h.following = nil
			h.followMu.Unlock()
			close(followed)
		}()
	}
	h.followMu.Unlock()
	select {
	case <-followed:
	case <-time.After(followWait):
	}
}

func serve[Q, A any](request []byte, op func(Q) A) []byte {
	var q Q
	if err := json.Unmarshal(request, &q); err != nil {
		return encode(outcome{Status: statusFailed})
	}
	return encode(op(q))
}

func encode(answer any) []byte {
	encoded, err := json.Marshal(answer)
	if err != nil {
		return []byte(`{"status":"failed"}`)
	}
	return encoded
}

func failure(err error) outcome {
	switch {
	case errors.Is(err, autofill.ErrLocked):
		return outcome{Status: statusLocked}
	case errors.Is(err, autofill.ErrNoCode):
		return outcome{Status: statusNoCode}
	case errors.Is(err, autofill.ErrPasskeyExcluded):
		return outcome{Status: statusExcluded}
	default:
		return outcome{Status: statusFailed}
	}
}

type suggestRequest struct {
	App    appWire `json:"app"`
	Fields []field `json:"fields"`
}

// suggestAnswer carries only the form while the vault is locked; Icons maps a site to a base64 PNG.
type suggestAnswer struct {
	outcome
	Form        *form             `json:"form,omitempty"`
	Suggestions []suggestionWire  `json:"suggestions,omitempty"`
	Icons       map[string]string `json:"icons,omitempty"`
	Search      bool              `json:"search"`
	Requester   *requesterWire    `json:"requester,omitempty"`
}

func (h *Handler) suggest(q suggestRequest) suggestAnswer {
	app, known := q.App.app()
	found, ok := formOf(q.Fields)
	if !known || !ok {
		return suggestAnswer{outcome: outcome{Status: statusNone}}
	}
	if !h.service.Open() {
		h.links.Forget()
		return h.locked(found)
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupWait)
	defer cancel()
	requester, err := h.requester(ctx, app, found)
	if err != nil {
		return h.failedSuggest(err, found)
	}
	answer := suggestAnswer{outcome: outcome{Status: statusOK}, Form: &found, Requester: requesterOf(requester)}
	if !found.fillable() {
		return answer
	}
	suggestions, err := h.service.Suggest(requester, found.Kind == formCode)
	if err != nil {
		return h.failedSuggest(err, found)
	}
	answer.Suggestions = suggestionsOf(suggestions)
	answer.Icons = h.iconsOf(suggestions)
	answer.Search = true
	return answer
}

// iconsOf spends the icon budget in suggestion order; a suggestion matched by a linked app has no site.
func (h *Handler) iconsOf(suggestions []autofill.Suggestion) map[string]string {
	var sites []string
	for _, s := range suggestions {
		if s.Site != "" && !slices.Contains(sites, s.Site) {
			sites = append(sites, s.Site)
		}
	}
	icons := make(map[string]string)
	budget := maxAnswerIconBytes
	for _, site := range sites {
		icon, err := h.icons.Cached(site)
		if err != nil {
			break
		}
		size := base64.StdEncoding.DecodedLen(len(icon))
		if icon != "" && size <= min(maxIconBytes, budget) {
			icons[site] = icon
			budget -= size
		}
	}
	return icons
}

func (h *Handler) failedSuggest(err error, found form) suggestAnswer {
	if errors.Is(err, autofill.ErrLocked) {
		return h.locked(found)
	}
	return suggestAnswer{outcome: failure(err)}
}

func (h *Handler) locked(found form) suggestAnswer {
	if !found.fillable() {
		return suggestAnswer{outcome: outcome{Status: statusNone}}
	}
	found.Save = nil
	return suggestAnswer{outcome: outcome{Status: statusLocked}, Form: &found}
}

// requester trusts the fields' web domain only from a trusted browser or an app the site's Asset Links name.
func (h *Handler) requester(ctx context.Context, app autofill.App, found form) (autofill.Requester, error) {
	origin := found.origin()
	if origin != "" && trustedBrowser(app) {
		return autofill.Requester{Origin: origin}, nil
	}
	sites, err := h.service.Sites()
	if err != nil {
		return autofill.Requester{}, err
	}
	if site := vault.SiteOf(origin); site != "" && slices.Contains(sites, site) &&
		len(h.links.Verified(ctx, []string{site}, app)) > 0 {
		return autofill.Requester{Origin: origin}, nil
	}
	return autofill.Requester{App: app, Sites: h.links.Verified(ctx, sites, app)}, nil
}

// origin is empty for a form without a web domain or one not over http or https; no scheme means https.
func (f form) origin() string {
	if f.domain == "" {
		return ""
	}
	origin, _ := vault.ParseOrigin(cmp.Or(f.scheme, "https") + "://" + f.domain)
	return origin
}

type fillRequest struct {
	Requester requesterWire `json:"requester"`
	ID        string        `json:"id"`
	Email     bool          `json:"email"`
}

type fillAnswer struct {
	outcome
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (h *Handler) fill(q fillRequest) fillAnswer {
	requester, ok := q.Requester.requester()
	if !ok {
		return fillAnswer{outcome: outcome{Status: statusFailed}}
	}
	login, err := h.service.Fill(q.ID, requester)
	if err != nil {
		return fillAnswer{outcome: failure(err)}
	}
	username := login.Login
	if q.Email && login.Email != "" || username == "" {
		username = login.Email
	}
	return fillAnswer{outcome: outcome{Status: statusOK}, Username: username, Password: login.Password}
}

// codeRequest's Fields counts the code form's fields.
type codeRequest struct {
	Requester requesterWire `json:"requester"`
	ID        string        `json:"id"`
	Fields    int           `json:"fields"`
}

type codeAnswer struct {
	outcome
	Entries []string `json:"entries,omitempty"`
}

func (h *Handler) code(q codeRequest) codeAnswer {
	requester, ok := q.Requester.requester()
	if !ok {
		return codeAnswer{outcome: outcome{Status: statusFailed}}
	}
	code, err := h.service.OneTimeCode(q.ID, requester)
	if err != nil {
		return codeAnswer{outcome: failure(err)}
	}
	return codeAnswer{outcome: outcome{Status: statusOK}, Entries: codeEntries(code.Code, q.Fields)}
}

type searchRequest struct {
	Requester requesterWire `json:"requester"`
	Query     string        `json:"query"`
	Code      bool          `json:"code"`
}

// searchResult's Matches is false for a credential the requester's site or app must first be added to.
type searchResult struct {
	suggestionWire
	Matches bool `json:"matches"`
}

type searchAnswer struct {
	outcome
	// Site must stay in ASCII form: a look-alike Unicode name would hide which site the owner adds.
	Site    string         `json:"site"`
	Scope   autofill.Scope `json:"scope"`
	Results []searchResult `json:"results"`
}

func (h *Handler) search(q searchRequest) searchAnswer {
	requester, ok := q.Requester.requester()
	if !ok {
		return searchAnswer{outcome: outcome{Status: statusFailed}}
	}
	listing, err := autofill.List(h.service, []autofill.Requester{requester}, q.Query, q.Code)
	if err != nil {
		return searchAnswer{outcome: failure(err)}
	}
	results := make([]searchResult, len(listing.Results))
	for i, result := range listing.Results {
		results[i] = searchResult{suggestionWire: suggestionOf(result.Suggestion), Matches: result.Matches}
	}
	return searchAnswer{outcome: outcome{Status: statusOK}, Site: pageSiteOf(requester), Scope: listing.Scope, Results: results}
}

func pageSiteOf(r autofill.Requester) string {
	if r.Origin == "" {
		return ""
	}
	return vaultservice.PageSite(r.Origin)
}

type linkRequest struct {
	Requester requesterWire `json:"requester"`
	ID        string        `json:"id"`
}

func (h *Handler) link(q linkRequest) outcome {
	requester, ok := q.Requester.requester()
	if !ok {
		return outcome{Status: statusFailed}
	}
	var err error
	if requester.Origin != "" {
		err = h.service.AddSite(q.ID, requester)
	} else {
		err = h.service.Link(q.ID, requester.App)
	}
	if err != nil {
		return failure(err)
	}
	return outcome{Status: statusOK}
}

type captureRequest struct {
	Requester requesterWire `json:"requester"`
	Account   string        `json:"account"`
	Password  string        `json:"password"`
}

// captureAnswer's Capture is the token the save screen asks for its sign-in by; Offer answers that screen alone.
type captureAnswer struct {
	outcome
	Offer   *offerWire `json:"offer,omitempty"`
	Capture string     `json:"capture,omitempty"`
}

// pendingSave is a sign-in the save screen asks for by token, with the offer made for it while the vault was open.
type pendingSave struct {
	capture autofill.Capture
	offer   *offerWire
}

// capture holds a sign-in in memory only and answers with a token alone, since the system keeps the save screen's
// intent: the offer names items and accounts of the vault.
func (h *Handler) capture(q captureRequest) captureAnswer {
	requester, ok := q.Requester.requester()
	if !ok || q.Password == "" {
		return captureAnswer{outcome: outcome{Status: statusFailed}}
	}
	pending := pendingSave{capture: autofill.Capture{Requester: requester, Account: q.Account, Password: q.Password}}
	status := statusLocked
	if h.service.Open() {
		answer := h.offer(pending.capture)
		switch answer.Status {
		case statusOK:
			pending.offer, status = answer.Offer, statusOK
		case statusLocked:
		default:
			return answer
		}
	}
	token, err := h.waiting.Keep(waitingSender, pending, nil)
	if err != nil {
		return captureAnswer{outcome: outcome{Status: statusFailed}}
	}
	return captureAnswer{outcome: outcome{Status: status}, Capture: token}
}

type offerRequest struct {
	Capture string `json:"capture"`
}

// offerWaiting answers the save screen with the offer its token names, once, and only while the vault is open, since
// the offer names its items; a sign-in waiting for the vault is offered once the vault opens.
func (h *Handler) offerWaiting(q offerRequest) captureAnswer {
	pending, ok := h.waiting.Find(waitingSender, q.Capture)
	if !ok {
		return captureAnswer{outcome: outcome{Status: statusFailed}}
	}
	answer := captureAnswer{outcome: outcome{Status: statusOK}, Offer: pending.offer}
	if pending.offer == nil || !h.service.Open() {
		answer = h.offer(pending.capture)
	}
	if answer.Status != statusLocked {
		h.waiting.Forget(waitingSender, q.Capture)
	}
	return answer
}

func (h *Handler) offer(captured autofill.Capture) captureAnswer {
	offer, err := h.service.Hold(captured)
	switch {
	case errors.Is(err, autofill.ErrNothingToSave):
		return captureAnswer{outcome: outcome{Status: statusNone}}
	case err != nil:
		return captureAnswer{outcome: failure(err)}
	}
	wire := offerOf(offer, captured)
	return captureAnswer{outcome: outcome{Status: statusOK}, Offer: &wire}
}

type saveRequest struct {
	Token  string     `json:"token"`
	Choice choiceWire `json:"choice"`
}

type saveAnswer struct {
	outcome
	Created bool `json:"created"`
}

func (h *Handler) save(q saveRequest) saveAnswer {
	created, err := h.service.Save(q.Token, autofill.Choice{Target: q.Choice.Target, Name: q.Choice.Name, Account: q.Choice.Account})
	switch {
	case errors.Is(err, autofill.ErrNameRefused):
		return saveAnswer{outcome: outcome{Status: statusNameRefused}}
	case errors.Is(err, autofill.ErrAccountRefused):
		return saveAnswer{outcome: outcome{Status: statusAccountRefused}}
	case err != nil:
		return saveAnswer{outcome: failure(err)}
	}
	return saveAnswer{outcome: outcome{Status: statusOK}, Created: created}
}

// methodsAnswer's Open means the unlock screen only confirms the owner.
type methodsAnswer struct {
	outcome
	Biometry     bool `json:"biometry"`
	PIN          bool `json:"pin"`
	AttemptsLeft int  `json:"attemptsLeft"`
	PINMin       int  `json:"pinMin"`
	PINMax       int  `json:"pinMax"`
	Open         bool `json:"open"`
}

func (h *Handler) methods() methodsAnswer {
	methods, err := h.vault.UnlockMethods()
	if err != nil {
		return methodsAnswer{outcome: failure(err)}
	}
	return methodsAnswer{
		outcome:      outcome{Status: statusOK},
		Biometry:     methods.BiometryEnabled && methods.BiometryAvailable,
		PIN:          methods.PINSet,
		AttemptsLeft: methods.PINAttemptsLeft,
		PINMin:       vault.MinPINLength,
		PINMax:       vault.MaxPINLength,
		Open:         h.service.Open(),
	}
}

type iconRequest struct {
	Site string `json:"site"`
}

type iconAnswer struct {
	outcome
	api.SiteIcon
}

func (h *Handler) icon(q iconRequest) iconAnswer {
	icon, err := h.page.SiteIcon(q.Site)
	if err != nil {
		return iconAnswer{outcome: failure(err)}
	}
	return iconAnswer{outcome: outcome{Status: statusOK}, SiteIcon: icon}
}

type languageAnswer struct {
	outcome
	api.LanguageSettings
}

func (h *Handler) language() languageAnswer {
	settings, err := h.page.GetLanguage()
	if err != nil {
		return languageAnswer{outcome: failure(err)}
	}
	return languageAnswer{outcome: outcome{Status: statusOK}, LanguageSettings: settings}
}

type interfaceSizeAnswer struct {
	outcome
	api.InterfaceSize
}

func (h *Handler) interfaceSize() interfaceSizeAnswer {
	size, err := h.page.GetInterfaceSize()
	if err != nil {
		return interfaceSizeAnswer{outcome: failure(err)}
	}
	return interfaceSizeAnswer{outcome: outcome{Status: statusOK}, InterfaceSize: size}
}

type appearanceAnswer struct {
	outcome
	api.Appearance
}

func (h *Handler) appearance() appearanceAnswer {
	appearance, err := h.page.GetAppearance()
	if err != nil {
		return appearanceAnswer{outcome: failure(err)}
	}
	return appearanceAnswer{outcome: outcome{Status: statusOK}, Appearance: appearance}
}

type unlockRequest struct {
	PIN string `json:"pin"`
}

type unlockAnswer struct {
	outcome
	AttemptsLeft int `json:"attemptsLeft,omitempty"`
}

func (h *Handler) unlock(q unlockRequest) unlockAnswer {
	if h.service.Open() {
		return unlockAnswer{outcome: outcome{Status: statusOK}}
	}
	var err error
	if q.PIN == "" {
		_, err = h.vault.Unlock(h.reason())
	} else {
		_, err = h.vault.UnlockWithPIN(q.PIN)
	}
	if err != nil {
		return h.refused(err)
	}
	h.opened()
	return unlockAnswer{outcome: outcome{Status: statusOK}}
}

func (h *Handler) refused(err error) unlockAnswer {
	switch {
	case errors.Is(err, ownerauth.ErrCanceled):
		return unlockAnswer{outcome: outcome{Status: statusCanceled}}
	case errors.Is(err, unlock.ErrWrongPIN), errors.Is(err, vault.ErrInvalidPIN):
		answer := unlockAnswer{outcome: outcome{Status: statusWrongPIN}}
		if methods, err := h.vault.UnlockMethods(); err == nil {
			answer.AttemptsLeft = methods.PINAttemptsLeft
		}
		return answer
	case errors.Is(err, unlock.ErrPINRemoved):
		return unlockAnswer{outcome: outcome{Status: statusPINRemoved}}
	case errors.Is(err, unlock.ErrTooSoon):
		return unlockAnswer{outcome: outcome{Status: statusTooSoon}}
	default:
		return unlockAnswer{outcome: outcome{Status: statusFailed}}
	}
}
