package customer

import (
	"errors"
	"reflect"
	"testing"

	"provider_saas/internal/platform/apperr"
)

func sp(s string) *string { return &s }

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
		check   func(t *testing.T, in CreateInput)
	}{
		{"ok normalises", CreateInput{Name: "  Karim  ", Phone: sp("+8801712345678"), Tags: []string{"VIP", " vip ", "netflix"}, Email: sp("  ")}, false,
			func(t *testing.T, in CreateInput) {
				if in.Name != "Karim" || *in.Phone != "01712345678" || in.Source != "manual" || in.Email != nil {
					t.Fatalf("normalised = %+v", in)
				}
				if !reflect.DeepEqual(in.Tags, []string{"vip", "netflix"}) {
					t.Fatalf("tags = %v", in.Tags)
				}
			}},
		{"missing name", CreateInput{Name: " "}, true, nil},
		{"bad source", CreateInput{Name: "a", Source: "tiktok"}, true, nil},
		{"bad channel", CreateInput{Name: "a", PreferredChannel: sp("pigeon")}, true, nil},
		{"upper channel", CreateInput{Name: "a", PreferredChannel: sp("WHATSAPP"), Source: "FACEBOOK"}, false,
			func(t *testing.T, in CreateInput) {
				if *in.PreferredChannel != "whatsapp" || in.Source != "facebook" {
					t.Fatalf("%+v", in)
				}
			}},
		{"bad email", CreateInput{Name: "a", Email: sp("nope")}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err != nil && !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("want validation error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, tt.in)
			}
		})
	}
}

func TestUpdateInputValidate(t *testing.T) {
	in := UpdateInput{Status: sp("BLOCKED"), Phone: sp(""), Tags: []string{"A"}}
	if err := in.Validate(); err != nil || *in.Status != "blocked" || *in.Phone != "" || in.Tags[0] != "a" {
		t.Fatalf("%+v %v", in, err)
	}
	if err := (&UpdateInput{Name: sp("")}).Validate(); err == nil {
		t.Fatal("empty name accepted")
	}
	if err := (&UpdateInput{Status: sp("vip")}).Validate(); err == nil {
		t.Fatal("bad status accepted")
	}
}
