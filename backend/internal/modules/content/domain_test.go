package content

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/modules/social"
)

func TestOverallStatus(t *testing.T) {
	p, f := Target{Status: TargetPublished}, Target{Status: TargetFailed}
	tests := []struct {
		in   []Target
		want string
	}{
		{[]Target{p, p}, StatusPublished}, {[]Target{p, f}, StatusPartial}, {[]Target{f, f}, StatusFailed}, {nil, StatusFailed},
	}
	for _, tt := range tests {
		if got := OverallStatus(tt.in); got != tt.want {
			t.Errorf("%v -> %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestPostInputValidate(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	in := PostInput{Body: " Eid offer! ", SocialAccountIDs: []uuid.UUID{id, id}, Media: []social.Attachment{{URL: "https://x.test/a.png"}}}
	if err := in.Validate(now); err != nil || in.Body != "Eid offer!" || len(in.SocialAccountIDs) != 1 || in.Media[0].Type != "image" {
		t.Fatalf("%+v %v", in, err)
	}
	past := now.Add(-time.Hour)
	for _, bad := range []PostInput{
		{Body: "", SocialAccountIDs: []uuid.UUID{id}},
		{Body: "x"},
		{Body: "x", SocialAccountIDs: []uuid.UUID{id}, ScheduledAt: &past},
		{Body: "x", SocialAccountIDs: []uuid.UUID{id}, Media: []social.Attachment{{URL: "file:///etc/passwd"}}},
	} {
		if err := bad.Validate(now); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestRenderPriceList(t *testing.T) {
	b, err := RenderPriceList("Demo Digital Store", []PriceRow{
		{Product: "Netflix Premium", Plans: []PricePlan{{"1 Month", takaLabel(35000, "BDT")}, {"3 Months", takaLabel(100000, "BDT")}}},
		{Product: "Canva Pro", Plans: []PricePlan{{"1 Year", takaLabel(90050, "BDT")}}},
	}, "footer")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dx() != 1080 || img.Bounds().Dy() < 500 {
		t.Fatalf("png: %v %v", img.Bounds(), err)
	}
	if takaLabel(100000, "BDT") != "Tk 1,000" || takaLabel(90050, "BDT") != "Tk 900.50" || takaLabel(500, "USD") != "5 USD" {
		t.Fatal("takaLabel")
	}
	if _, err := RenderPriceList("x", nil, ""); err == nil {
		t.Fatal("empty list accepted")
	}
}
