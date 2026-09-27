package autofill

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/dortanes/ravenpass/packages/app/autofill/fieldwords"
)

// field is a text field as Java read it from the view structure; answers refer to it by Index.
type field struct {
	Index     int      `json:"index"`
	Hints     []string `json:"hints"`
	InputType int      `json:"inputType"`
	// MaxLength is a native field's length filter; Chrome reports a page's as the maxlength attribute.
	MaxLength int      `json:"maxLength"`
	HTML      htmlInfo `json:"html"`
	// WebDomain and WebScheme come from the nearest node that declares a web domain.
	WebDomain string `json:"webDomain"`
	WebScheme string `json:"webScheme"`
	IDEntry   string `json:"idEntry"`
	Hint      string `json:"hint"`
	Focused   bool   `json:"focused"`
	Visible   bool   `json:"visible"`
}

// htmlInfo is a web field's tag and attributes, with the attribute names in lowercase.
type htmlInfo struct {
	Tag        string            `json:"tag"`
	Attributes map[string]string `json:"attributes"`
}

type role uint8

const (
	roleNone role = iota
	roleLogin
	rolePassword
	roleNewPassword
	roleCode
)

type kind uint8

const (
	kindOther kind = iota
	kindText
	kindEmail
	kindPassword
	kindNumber
	kindTel
	kindURL
	kindSearch
)

// Bits of android.text.InputType.
const (
	inputClassMask               = 0x00f
	inputClassNone               = 0x000
	inputClassText               = 0x001
	inputClassNumber             = 0x002
	inputClassPhone              = 0x003
	inputVariationMask           = 0xff0
	textVariationURI             = 0x010
	textVariationEmail           = 0x020
	textVariationPassword        = 0x080
	textVariationVisiblePassword = 0x090
	textVariationWebEmail        = 0x0d0
	textVariationWebPassword     = 0x0e0
	numberVariationPassword      = 0x010
)

var codeLengths = struct{ min, max int }{4, 10}

// splitRunLength is the fewest boxes in a row that make one split code field, matching the browser extension.
const splitRunLength = 4

var terms = fieldwords.Load()

// chromePredictions are Chrome's field-type attributes, most decisive first.
var chromePredictions = []string{"computed-autofill-hints", "crowdsourcing-autofill-hints", "ua-autofill-hints"}

func classify(f field, precedesPassword bool) role {
	k := f.kind()
	if !f.visible() || !k.typed() {
		return roleNone
	}
	words := f.words()
	if mentions(words, terms.Search) {
		return roleNone
	}
	if declared, ok := f.declared(); ok {
		return declared
	}
	switch k {
	case kindPassword:
		if mentions(words, terms.NewPassword) {
			return roleNewPassword
		}
		return rolePassword
	case kindEmail:
		return roleLogin
	}
	// The code check precedes the login check: a code field's label may name the email the code went to.
	if k.code() && (mentions(words, terms.OneTime) || (f.numeric() || f.takesCode()) && mentions(words, terms.Code)) {
		return roleCode
	}
	if k == kindText && mentions(words, terms.Login) {
		return roleLogin
	}
	if k == kindText && precedesPassword {
		return roleLogin
	}
	return roleNone
}

func (f field) asksEmail() bool {
	if f.kind() == kindEmail {
		return true
	}
	for _, token := range f.declaredTokens() {
		switch strings.ToLower(token) {
		case "email", "emailaddress":
			return true
		}
	}
	for _, name := range chromePredictions {
		if slices.ContainsFunc(tokensOf(f.HTML.Attributes[name]), func(token string) bool {
			return strings.EqualFold(token, "EMAIL_ADDRESS")
		}) {
			return true
		}
	}
	return false
}

func (f field) kind() kind {
	if strings.EqualFold(f.HTML.Tag, "input") {
		switch strings.ToLower(f.HTML.Attributes["type"]) {
		case "", "text":
			return kindText
		case "email":
			return kindEmail
		case "password":
			return kindPassword
		case "number":
			return kindNumber
		case "tel":
			return kindTel
		case "url":
			return kindURL
		case "search":
			return kindSearch
		default:
			return kindOther
		}
	}
	variation := f.InputType & inputVariationMask
	switch f.InputType & inputClassMask {
	case inputClassNone:
		// Custom text views and some browsers' fields declare no input type.
		return kindText
	case inputClassText:
		switch variation {
		case textVariationPassword, textVariationVisiblePassword, textVariationWebPassword:
			return kindPassword
		case textVariationEmail, textVariationWebEmail:
			return kindEmail
		case textVariationURI:
			return kindURL
		default:
			return kindText
		}
	case inputClassNumber:
		if variation == numberVariationPassword {
			return kindPassword
		}
		return kindNumber
	case inputClassPhone:
		return kindTel
	default:
		return kindOther
	}
}

func (k kind) typed() bool {
	switch k {
	case kindText, kindEmail, kindPassword, kindNumber, kindTel, kindURL:
		return true
	default:
		return false
	}
}

func (k kind) code() bool { return k == kindText || k == kindNumber || k == kindTel }

func (f field) visible() bool {
	return f.Visible && !strings.EqualFold(f.HTML.Attributes["visibility"], "invisible")
}

// numeric sees a page's field only by its type: Chrome leaves inputmode out of a page's fields.
func (f field) numeric() bool {
	k := f.kind()
	return k == kindNumber || k == kindTel || strings.EqualFold(f.HTML.Attributes["inputmode"], "numeric")
}

func (f field) takesCode() bool {
	length, ok := f.maxLength()
	return ok && length >= codeLengths.min && length <= codeLengths.max
}

func (f field) maxLength() (int, bool) {
	if length, err := strconv.Atoi(f.HTML.Attributes["maxlength"]); err == nil {
		return length, true
	}
	return f.MaxLength, f.MaxLength > 0
}

func (f field) box() bool {
	if !f.visible() || !f.kind().code() {
		return false
	}
	length, ok := f.maxLength()
	return ok && length == 1 || slices.ContainsFunc(f.Hints, characterHint)
}

// characterHint reports Android's per-character SMS code hint: smsOTPCode followed by a position.
func characterHint(hint string) bool {
	position, found := strings.CutPrefix(strings.ToLower(hint), "smsotpcode")
	_, err := strconv.Atoi(position)
	return found && err == nil
}

func (f field) declared() (role, bool) {
	for _, token := range f.declaredTokens() {
		if declared, ok := declaredRole(token); ok {
			return declared, true
		}
	}
	for _, name := range chromePredictions {
		for _, token := range tokensOf(f.HTML.Attributes[name]) {
			if predicted, ok := predictedRole(token); ok {
				return predicted, true
			}
		}
	}
	return roleNone, false
}

// declaredTokens: Chrome copies a page's autocomplete tokens into the field's Android hints.
func (f field) declaredTokens() []string {
	return append(append([]string{}, f.Hints...), strings.Fields(f.HTML.Attributes["autocomplete"])...)
}

// declaredRole reads an Android autofill hint or a W3C autocomplete token.
func declaredRole(token string) (role, bool) {
	token = strings.ToLower(token)
	switch {
	case token == "username", token == "newusername", token == "email", token == "emailaddress":
		return roleLogin, true
	case token == "password", token == "current-password":
		return rolePassword, true
	case token == "newpassword", token == "new-password":
		return roleNewPassword, true
	case token == "one-time-code", token == "emailotpcode", token == "2faappotpcode",
		strings.HasPrefix(token, "smsotpcode"):
		return roleCode, true
	case token == "notapplicable":
		return roleNone, true
	default:
		return roleNone, false
	}
}

// predictedRole reads one of Chrome's field types.
func predictedRole(token string) (role, bool) {
	switch strings.ToUpper(token) {
	case "USERNAME", "USERNAME_AND_EMAIL_ADDRESS", "SINGLE_USERNAME", "EMAIL_ADDRESS":
		return roleLogin, true
	case "PASSWORD":
		return rolePassword, true
	case "ACCOUNT_CREATION_PASSWORD", "NEW_PASSWORD":
		return roleNewPassword, true
	case "CONFIRMATION_PASSWORD":
		return roleNone, true
	case "ONE_TIME_CODE":
		return roleCode, true
	default:
		return roleNone, false
	}
}

// tokensOf splits one of Chrome's attributes, which may hold several types.
func tokensOf(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

func (f field) words() []string {
	parts := []string{f.IDEntry, f.Hint}
	for _, name := range []string{"name", "id", "label", "placeholder", "aria-label"} {
		parts = append(parts, f.HTML.Attributes[name])
	}
	return wordsIn(strings.Join(parts, " "))
}

var caseChange = regexp.MustCompile(`(\p{Ll}|\p{N})(\p{Lu})`)

func wordsIn(text string) []string {
	return strings.FieldsFunc(strings.ToLower(caseChange.ReplaceAllString(text, "$1 $2")), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// mentions matches a term's words in a row, the last as a prefix and the others exactly.
func mentions(words []string, terms []string) bool {
	for _, term := range terms {
		parts := strings.Split(term, " ")
		last := len(parts) - 1
		for start := range words {
			if start+last >= len(words) {
				break
			}
			matched := true
			for offset, part := range parts {
				word := words[start+offset]
				if offset == last && !strings.HasPrefix(word, part) || offset != last && word != part {
					matched = false
					break
				}
			}
			if matched {
				return true
			}
		}
	}
	return false
}
