package importers

import "testing"

func TestBlankIsWhiteSpaceAlone(t *testing.T) {
	for value, want := range map[string]bool{"": true, " \t\r\n": true, "a": false, " a ": false} {
		if got := Blank(value); got != want {
			t.Errorf("Blank(%q) = %v", value, got)
		}
	}
}

func TestJoinedKeepsThePartsThatAreNotBlank(t *testing.T) {
	tests := []struct {
		separator string
		parts     []string
		want      string
	}{
		{" ", []string{" Alex ", "", "  ", "Doe"}, "Alex Doe"},
		{"/", []string{"12", "2027"}, "12/2027"},
		{"/", []string{"", "2027"}, "2027"},
		{", ", nil, ""},
	}
	for _, test := range tests {
		if got := Joined(test.separator, test.parts...); got != test.want {
			t.Errorf("Joined(%q, %q) = %q, want %q", test.separator, test.parts, got, test.want)
		}
	}
}

func TestIsEmailTakesAnAddressAlone(t *testing.T) {
	tests := map[string]bool{
		"alex@example.test":         true,
		"Alex <alex@example.test>":  false,
		" alex@example.test":        false,
		"alex":                      false,
		"alex@":                     false,
		"":                          false,
		"alex.doe+tag@mail.example": true,
	}
	for value, want := range tests {
		if got := IsEmail(value); got != want {
			t.Errorf("IsEmail(%q) = %v", value, got)
		}
	}
}
