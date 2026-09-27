package autofill

import (
	"slices"
	"strings"
)

type formKind string

const (
	formSignIn formKind = "sign-in"
	formCode   formKind = "code"
	// formSave is a new password field: nothing to fill, only a sign-up to save.
	formSave formKind = "save"
)

// form refers to fields by Java's indexes, -1 for none.
type form struct {
	Kind     formKind    `json:"kind"`
	Username int         `json:"username"`
	Password int         `json:"password"`
	Code     []int       `json:"code,omitempty"`
	Email    bool        `json:"email"`
	Save     *saveFields `json:"save,omitempty"`
	domain   string
	scheme   string
}

type saveFields struct {
	Username int `json:"username"`
	Password int `json:"password"`
}

func (f form) fillable() bool { return f.Kind != formSave }

// formOf keeps to the focused field's web domain, so a page's and an app's fields never share a form.
func formOf(fields []field) (form, bool) {
	at := slices.IndexFunc(fields, func(f field) bool { return f.Focused })
	if at < 0 {
		return form{}, false
	}
	focused := fields[at]
	var scope []field
	for _, f := range fields {
		if f.WebDomain == focused.WebDomain {
			scope = append(scope, f)
		}
	}
	position := slices.IndexFunc(scope, func(f field) bool { return f.Index == focused.Index })
	kinds := roles(scope)
	index := func(position int) int {
		if position < 0 {
			return -1
		}
		return scope[position].Index
	}
	found := form{Username: -1, Password: -1, domain: focused.WebDomain, scheme: focused.WebScheme}
	switch kinds[position] {
	case roleCode:
		found.Kind = formCode
		boxes := splitRun(scope, position)
		if boxes == nil {
			boxes = []int{position}
		}
		for _, box := range boxes {
			found.Code = append(found.Code, index(box))
		}
	case roleLogin, rolePassword:
		found.Kind = formSignIn
		login, password := position, position
		if kinds[position] == roleLogin {
			password = next(kinds, position, rolePassword)
			if password >= 0 && last(kinds, password, roleLogin) != position {
				password = -1
			}
		} else {
			login = last(kinds, position, roleLogin)
		}
		found.Username, found.Password = index(login), index(password)
		found.Email = login >= 0 && scope[login].asksEmail()
		saved := password
		if saved < 0 && login >= 0 {
			saved = next(kinds, login, roleNewPassword)
		}
		if saved >= 0 {
			found.Save = &saveFields{Username: index(login), Password: index(saved)}
		}
	case roleNewPassword:
		found.Kind = formSave
		login := last(kinds, position, roleLogin)
		found.Email = login >= 0 && scope[login].asksEmail()
		found.Save = &saveFields{Username: index(login), Password: focused.Index}
	default:
		return form{}, false
	}
	return found, true
}

func roles(scope []field) []role {
	var typed []int
	for i, f := range scope {
		if f.visible() && f.kind().sequenced() {
			typed = append(typed, i)
		}
	}
	kinds := make([]role, len(scope))
	for i, f := range scope {
		at := slices.Index(typed, i)
		precedes := at >= 0 && at+1 < len(typed) && scope[typed[at+1]].kind() == kindPassword
		kinds[i] = classify(f, precedes)
		if kinds[i] != roleLogin && kinds[i] != rolePassword && splitRun(scope, i) != nil {
			kinds[i] = roleCode
		}
	}
	return kinds
}

func splitRun(scope []field, at int) []int {
	if !scope[at].box() {
		return nil
	}
	start, end := at, at+1
	for start > 0 && scope[start-1].box() {
		start--
	}
	for end < len(scope) && scope[end].box() {
		end++
	}
	if end-start < splitRunLength {
		return nil
	}
	run := make([]int, 0, end-start)
	for position := start; position < end; position++ {
		run = append(run, position)
	}
	return run
}

// codeEntries splits a code across fields as the browser extension does.
func codeEntries(code string, fields int) []string {
	if characters := strings.Split(code, ""); len(characters) == fields {
		return characters
	}
	return []string{code}
}

// sequenced mirrors the browser extension's input types whose order pairs a login with its password.
func (k kind) sequenced() bool {
	return k == kindText || k == kindEmail || k == kindPassword || k == kindTel || k == kindURL
}

func next(kinds []role, from int, r role) int {
	for i := from + 1; i < len(kinds); i++ {
		if kinds[i] == r {
			return i
		}
	}
	return -1
}

func last(kinds []role, until int, r role) int {
	for i := until - 1; i >= 0; i-- {
		if kinds[i] == r {
			return i
		}
	}
	return -1
}
