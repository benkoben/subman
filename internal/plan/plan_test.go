package plan

import "testing"

func TestNewPlan(t *testing.T) {
	description := "Two screens, HD"
	tests := []struct {
		name          string
		planName      string
		vendor        string
		description   *string
		frequency     BillingFrequency
		wantErr       bool
		wantFrequency BillingFrequency
	}{
		{
			name:          "valid plan",
			planName:      "Netflix Premium",
			vendor:        "Netflix",
			description:   &description,
			frequency:     Monthly,
			wantErr:       false,
			wantFrequency: Monthly,
		},
		{
			name:          "empty name",
			planName:      "",
			vendor:        "Netflix",
			description:   nil,
			frequency:     Annually,
			wantErr:       true,
			wantFrequency: "",
		},
		{
			name:          "empty vendor",
			planName:      "Netflix Premium",
			vendor:        "",
			description:   nil,
			frequency:     Monthly,
			wantErr:       true,
			wantFrequency: "",
		},
		{
			name:          "unknown frequency",
			planName:      "Netflix Premium",
			vendor:        "Netflix",
			description:   nil,
			frequency:     BillingFrequency("weekly"),
			wantErr:       true,
			wantFrequency: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPlan(tt.planName, tt.vendor, tt.description, tt.frequency)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewPlan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Name != tt.planName {
				t.Errorf("Name = %q, want %q", got.Name, tt.planName)
			}
			if got.Vendor != tt.vendor {
				t.Errorf("Vendor = %q, want %q", got.Vendor, tt.vendor)
			}
			if got.Description != tt.description {
				t.Errorf("Description = %v, want %v", got.Description, tt.description)
			}
			if got.BillingFrequency != tt.wantFrequency {
				t.Errorf("BillingFrequency = %q, want %q", got.BillingFrequency, tt.wantFrequency)
			}
		})
	}
}
