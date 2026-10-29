package catalog

import "testing"

func TestCreateInputValidate(t *testing.T) {
	sp := func(s string) *string { return &s }
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{"defaults", CreateInput{Name: "Netflix", Category: sp(" Streaming ")}, false},
		{"empty name", CreateInput{Name: ""}, true},
		{"bad delivery", CreateInput{Name: "x", DeliveryType: "teleport"}, true},
		{"upper delivery", CreateInput{Name: "x", DeliveryType: "LICENSE_KEY"}, false},
		{"bad plan duration", CreateInput{Name: "x", Plans: []PlanInput{{Name: "1m", DurationDays: 0, Price: 100}}}, true},
		{"negative price", CreateInput{Name: "x", Plans: []PlanInput{{Name: "1m", DurationDays: 30, Price: -1}}}, true},
		{"negative warranty", CreateInput{Name: "x", WarrantyDays: -1}, true},
		{"ok plan", CreateInput{Name: "x", Plans: []PlanInput{{Name: " 1 Month ", DurationDays: 30, Price: 35000, Cost: 20000}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err == nil && tt.in.DeliveryType == "" {
				t.Fatal("delivery default not applied")
			}
		})
	}
	in := CreateInput{Name: "x", Category: sp(" Streaming ")}
	_ = in.Validate()
	if *in.Category != "streaming" || in.DeliveryType != DeliverySharedSlot {
		t.Fatalf("%+v", in)
	}
}
