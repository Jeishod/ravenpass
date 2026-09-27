package autofill

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

var roleNames = map[role]string{
	roleNone:        "none",
	roleLogin:       "login",
	rolePassword:    "password",
	roleNewPassword: "new-password",
	roleCode:        "code",
}

func TestFieldsClassifyAsTheFixtureTableSays(t *testing.T) {
	data, err := os.ReadFile("testdata/fields.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string   `json:"name"`
		Fields []field  `json:"fields"`
		Roles  []string `json:"roles"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("the fixture table holds no case")
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			var got []string
			for _, r := range roles(test.Fields) {
				got = append(got, roleNames[r])
			}
			if !slices.Equal(got, test.Roles) {
				t.Fatalf("roles %v, want %v", got, test.Roles)
			}
		})
	}
}

func TestALoginFieldAsksForAnEmailWhenItsTypeOrDeclarationSaysSo(t *testing.T) {
	for _, test := range []struct {
		name string
		f    field
		want bool
	}{
		{"email input type", field{InputType: inputClassText | textVariationEmail}, true},
		{"Android email hint", field{Hints: []string{"emailAddress"}}, true},
		{"autocomplete email", field{Hints: []string{"email"}}, true},
		{"Chrome's email prediction", field{HTML: htmlInfo{Tag: "input", Attributes: map[string]string{"computed-autofill-hints": "EMAIL_ADDRESS"}}}, true},
		{"plain username", field{Hints: []string{"username"}, InputType: inputClassText}, false},
	} {
		if got := test.f.asksEmail(); got != test.want {
			t.Errorf("%s: asks for an email %v, want %v", test.name, got, test.want)
		}
	}
}

func TestFieldNamingMentionsTheTermListsTheSharedTableSays(t *testing.T) {
	data, err := os.ReadFile("../../../../packages/app/autofill/fieldwords/testdata/matches.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text     string   `json:"text"`
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("the shared table holds no case")
	}
	lists := []struct {
		name  string
		terms []string
	}{
		{"search", terms.Search}, {"newPassword", terms.NewPassword}, {"login", terms.Login},
		{"oneTime", terms.OneTime}, {"code", terms.Code},
	}
	for _, test := range cases {
		got := []string{}
		for _, list := range lists {
			if mentions(wordsIn(test.Text), list.terms) {
				got = append(got, list.name)
			}
		}
		if !slices.Equal(got, test.Mentions) {
			t.Errorf("%q mentions %v, want %v", test.Text, got, test.Mentions)
		}
	}
}

func TestWordsSplitAtCaseChangesAndPunctuation(t *testing.T) {
	got := wordsIn("loginField user_name2FA Почта-адрес")
	want := []string{"login", "field", "user", "name2", "fa", "почта", "адрес"}
	if !slices.Equal(got, want) {
		t.Fatalf("words %q, want %q", got, want)
	}
	if !mentions([]string{"enter", "one", "time", "password"}, []string{"one time"}) {
		t.Fatal("a term of two words did not match them in a row")
	}
	if mentions([]string{"time", "one"}, []string{"one time"}) {
		t.Fatal("a term of two words matched them out of order")
	}
}
