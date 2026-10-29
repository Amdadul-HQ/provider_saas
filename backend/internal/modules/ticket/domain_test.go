package ticket

import (
	"testing"

	"github.com/google/uuid"
)

func TestCreateInputValidate(t *testing.T) {
	sub := uuid.New()
	in := CreateInput{CustomerID: uuid.New(), SubscriptionID: &sub, Kind: "REPLACEMENT", Title: " Netflix not working "}
	if err := in.Validate(); err != nil || in.Kind != KindReplacement || in.Priority != "normal" || in.Title != "Netflix not working" {
		t.Fatalf("%+v %v", in, err)
	}
	for _, bad := range []CreateInput{
		{Kind: "replacement", Title: "no sub"}, {Kind: "bogus", Title: "x"}, {Kind: "question", Title: ""},
		{Kind: "question", Title: "x", Priority: "asap"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	for in, want := range map[string]string{"": "banned", "BANNED": "banned", "retired": "retired", "free": "free"} {
		if got, err := ValidOldSlotStatus(in); err != nil || got != want {
			t.Errorf("%q -> %q %v", in, got, err)
		}
	}
	if _, err := ValidOldSlotStatus("assigned"); err == nil {
		t.Fatal("assigned accepted for old slot")
	}
}
