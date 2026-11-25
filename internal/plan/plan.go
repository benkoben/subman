package plan

import "fmt"

// BillingFrequency is how often a plan charges: monthly or annually.
type BillingFrequency string

const (
	Monthly  BillingFrequency = "monthly"
	Annually BillingFrequency = "annually"
)

// Plan is the user-entered description of what is subscribed to.
// It is a value object owned by its Subscription and carries no price:
// billing amounts live in the Subscription's price intervals.
type Plan struct {
	Name        string
	Vendor      string
	Description *string

	BillingFrequency BillingFrequency
}

// NewPlan validates and constructs a Plan. It errors on an empty name,
// an empty vendor, or a billing frequency other than Monthly or Annually.
func NewPlan(name, vendor string, description *string, bf BillingFrequency) (Plan, error) {
	var p Plan

	if name == "" {
		return p, fmt.Errorf("plan name cannot be empty")
	}
	if vendor == "" {
		return p, fmt.Errorf("plan vendor cannot be empty")
	}
	if bf != Monthly && bf != Annually {
		return p, fmt.Errorf("billing frequency must be %q or %q, got %q", Monthly, Annually, bf)
	}

	p.Name = name
	p.Vendor = vendor
	p.Description = description
	p.BillingFrequency = bf

	return p, nil
}
