package auth

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"0500000000":       "+966500000000",
		"05 0000 0000":     "+966500000000",
		"500000000":        "+966500000000",
		"966500000000":     "+966500000000",
		"+966 50 000 0000": "+966500000000",
		"00966500000000":   "+966500000000",
		"٠٥٠٠٠٠٠٠٠٠":       "+966500000000",
		"+971501234567":    "+971501234567",
	}
	for in, want := range cases {
		got, err := NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "12345", "0600000000", "+96650000000", "abc"} {
		if _, err := NormalizePhone(bad); err == nil {
			t.Errorf("NormalizePhone(%q) accepted", bad)
		}
	}
}

func TestRegisterTicket(t *testing.T) {
	h := &Handler{Secret: []byte("test-secret-test-secret-test-secret")}
	ticket, err := h.registerTicket("+966500000000")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.checkTicket(ticket, "+966500000000"); err != nil {
		t.Errorf("valid ticket refused: %v", err)
	}
	if h.checkTicket(ticket, "+966500000001") == nil {
		t.Error("ticket accepted for another phone")
	}
	other := &Handler{Secret: []byte("another-secret-another-secret-xx")}
	if other.checkTicket(ticket, "+966500000000") == nil {
		t.Error("ticket accepted under another key")
	}
	session, _ := h.Token("dev-owner", "11111111-1111-4111-8111-111111111111")
	if h.checkTicket(session, "+966500000000") == nil {
		t.Error("a session token was accepted as a registration ticket")
	}
}

func TestCodeHashIsBoundToPhone(t *testing.T) {
	if hashCode("+966500000000", "123456") == hashCode("+966500000001", "123456") {
		t.Error("the same code must hash differently for different phones")
	}
	c, err := newCode()
	if err != nil || len(c) != 6 {
		t.Errorf("newCode = %q, %v", c, err)
	}
}
