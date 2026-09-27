package autofill

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

func text(index int, id string) field {
	return field{Index: index, InputType: inputClassText, IDEntry: id, Visible: true}
}

func password(index int, id string) field {
	return field{Index: index, InputType: inputClassText | textVariationPassword, IDEntry: id, Visible: true}
}

func focus(f field) field {
	f.Focused = true
	return f
}

func onPage(f field, domain string) field {
	f.WebDomain = domain
	f.WebScheme = "https"
	return f
}

func pageInput(index int, attributes map[string]string) field {
	return field{Index: index, HTML: htmlInfo{Tag: "input", Attributes: attributes}, Visible: true}
}

// splitBoxes are count one-character digit boxes numbered from first, the one at position focused focused.
func splitBoxes(first, count, focused int) []field {
	run := make([]field, count)
	for i := range run {
		run[i] = field{Index: first + i, InputType: inputClassNumber, MaxLength: 1, Visible: true}
	}
	run[focused] = focus(run[focused])
	return run
}

func TestTheFocusedFieldPicksItsForm(t *testing.T) {
	email := field{Index: 1, InputType: inputClassText | textVariationEmail, Visible: true}
	for _, test := range []struct {
		name   string
		fields []field
		want   form
	}{
		{
			name:   "password pairs with the login before it",
			fields: []field{text(1, "login"), focus(password(2, "password"))},
			want:   form{Kind: formSignIn, Username: 1, Password: 2, Save: &saveFields{Username: 1, Password: 2}},
		},
		{
			name:   "login pairs with the password after it",
			fields: []field{focus(text(1, "login")), password(2, "password"), text(3, "promo")},
			want:   form{Kind: formSignIn, Username: 1, Password: 2, Save: &saveFields{Username: 1, Password: 2}},
		},
		{
			name:   "a login page without a password saves nothing",
			fields: []field{focus(text(1, "login"))},
			want:   form{Kind: formSignIn, Username: 1, Password: -1},
		},
		{
			name:   "a login another login follows keeps no password",
			fields: []field{focus(text(1, "login")), text(2, "user_login"), password(3, "password")},
			want:   form{Kind: formSignIn, Username: 1, Password: -1},
		},
		{
			name:   "an email login asks for the email",
			fields: []field{focus(email), password(2, "password")},
			want:   form{Kind: formSignIn, Username: 1, Password: 2, Email: true, Save: &saveFields{Username: 1, Password: 2}},
		},
		{
			name:   "a sign-up login fills the login and saves the new password",
			fields: []field{focus(text(1, "login")), password(2, "new_password")},
			want:   form{Kind: formSignIn, Username: 1, Password: -1, Save: &saveFields{Username: 1, Password: 2}},
		},
		{
			name:   "a new password only saves",
			fields: []field{text(1, "login"), focus(password(2, "new_password"))},
			want:   form{Kind: formSave, Username: -1, Password: -1, Save: &saveFields{Username: 1, Password: 2}},
		},
		{
			name:   "a code field stands alone",
			fields: []field{text(1, "login"), focus(field{Index: 2, InputType: inputClassNumber, IDEntry: "otp", Visible: true})},
			want:   form{Kind: formCode, Username: -1, Password: -1, Code: []int{2}},
		},
		{
			name:   "a page whose only field takes the code",
			fields: []field{focus(onPage(pageInput(4, map[string]string{"type": "tel", "name": "otpCode", "maxlength": "20"}), "example.com"))},
			want:   form{Kind: formCode, Username: -1, Password: -1, Code: []int{4}, domain: "example.com", scheme: "https"},
		},
		{
			name:   "any box of a split code field fills every box in order",
			fields: append([]field{text(0, "login")}, splitBoxes(1, 6, 3)...),
			want:   form{Kind: formCode, Username: -1, Password: -1, Code: []int{1, 2, 3, 4, 5, 6}},
		},
		{
			name: "a page's split code field of four boxes",
			fields: []field{
				focus(onPage(pageInput(0, map[string]string{"type": "text", "maxlength": "1"}), "example.com")),
				onPage(pageInput(1, map[string]string{"type": "text", "maxlength": "1"}), "example.com"),
				onPage(pageInput(2, map[string]string{"type": "tel", "maxlength": "1"}), "example.com"),
				onPage(pageInput(3, map[string]string{"type": "number", "maxlength": "1"}), "example.com"),
			},
			want: form{Kind: formCode, Username: -1, Password: -1, Code: []int{0, 1, 2, 3}, domain: "example.com", scheme: "https"},
		},
		{
			name: "a box in a run a hidden box breaks is a code field alone",
			fields: func() []field {
				run := splitBoxes(0, 7, 4)
				for i := range run {
					run[i].IDEntry = "otp_digit"
				}
				run[3].Visible = false
				return run
			}(),
			want: form{Kind: formCode, Username: -1, Password: -1, Code: []int{4}},
		},
		{
			name:   "a page's password does not pair with the app's own login",
			fields: []field{text(1, "login"), focus(onPage(password(2, "password"), "example.com"))},
			want:   form{Kind: formSignIn, Username: -1, Password: 2, Save: &saveFields{Username: -1, Password: 2}, domain: "example.com", scheme: "https"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := formOf(test.fields)
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("form %+v (%v), want %+v", got, ok, test.want)
			}
		})
	}
}

func TestNoFormWithoutAFocusedSignInOrCodeField(t *testing.T) {
	for name, fields := range map[string][]field{
		"nothing focused":       {text(1, "login"), password(2, "password")},
		"focused field is none": {focus(text(1, "comment")), text(2, "subject")},
		"three unnamed boxes":   splitBoxes(0, 3, 1),
	} {
		if found, ok := formOf(fields); ok {
			t.Errorf("%s: found %+v", name, found)
		}
	}
}

// codeTable is the code field table the browser extension's tests read too.
type codeTable struct {
	Fields []struct {
		Name   string `json:"name"`
		Inputs []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Label     string `json:"label"`
			InputMode string `json:"inputMode"`
			MaxLength *int   `json:"maxLength"`
			Visible   *bool  `json:"visible"`
		} `json:"inputs"`
		Focus int   `json:"focus"`
		Code  []int `json:"code"`
	} `json:"fields"`
	Entries []struct {
		Code    string   `json:"code"`
		Boxes   int      `json:"boxes"`
		Entries []string `json:"entries"`
	} `json:"entries"`
}

func sharedCodeTable(t *testing.T) codeTable {
	t.Helper()
	data, err := os.ReadFile("../../../../packages/app/autofill/fieldwords/testdata/codes.json")
	if err != nil {
		t.Fatal(err)
	}
	var table codeTable
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Fields) == 0 || len(table.Entries) == 0 {
		t.Fatal("the shared code table holds no case")
	}
	return table
}

func TestPageCodeFieldsAreFoundAsTheSharedTableSays(t *testing.T) {
	for _, test := range sharedCodeTable(t).Fields {
		t.Run(test.Name, func(t *testing.T) {
			fields := make([]field, len(test.Inputs))
			for i, input := range test.Inputs {
				attributes := map[string]string{"type": input.Type, "name": input.Name, "label": input.Label, "inputmode": input.InputMode}
				if input.MaxLength != nil {
					attributes["maxlength"] = strconv.Itoa(*input.MaxLength)
				}
				fields[i] = pageInput(i, attributes)
				fields[i].Visible = input.Visible == nil || *input.Visible
			}
			fields[test.Focus] = focus(fields[test.Focus])
			var got []int
			if found, ok := formOf(fields); ok && found.Kind == formCode {
				got = found.Code
			}
			if !slices.Equal(got, test.Code) {
				t.Fatalf("code fields %v, want %v", got, test.Code)
			}
		})
	}
}

func TestACodeFillsItsFieldsAsTheSharedTableSays(t *testing.T) {
	for _, test := range sharedCodeTable(t).Entries {
		if got := codeEntries(test.Code, test.Boxes); !slices.Equal(got, test.Entries) {
			t.Errorf("%s over %d fields: %q, want %q", test.Code, test.Boxes, got, test.Entries)
		}
	}
}

func TestAFormsOriginIsItsWebDomainOverHTTP(t *testing.T) {
	for _, test := range []struct {
		domain, scheme, want string
	}{
		{"example.com", "https", "https://example.com"},
		{"example.com", "", "https://example.com"},
		{"example.com", "HTTP", "http://example.com"},
		{"example.com", "file", ""},
		{"", "https", ""},
	} {
		if got := (form{domain: test.domain, scheme: test.scheme}).origin(); got != test.want {
			t.Errorf("%s over %q: origin %q, want %q", test.domain, test.scheme, got, test.want)
		}
	}
}
