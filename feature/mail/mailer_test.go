package mail

import "testing"

func TestMessage_Validate(t *testing.T) {
	cases := []struct {
		name    string
		msg     Message
		wantErr bool
	}{
		{"text ok", Message{To: []string{"a@b.com"}, Text: "x"}, false},
		{"html ok", Message{To: []string{"a@b.com"}, HTML: "<b>x</b>"}, false},
		{"both ok", Message{To: []string{"a@b.com"}, Text: "x", HTML: "<b>x</b>"}, false},
		{"no recipients", Message{Text: "x"}, true},
		{"empty body", Message{To: []string{"a@b.com"}}, true},
	}
	for _, c := range cases {
		err := c.msg.validate()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}
