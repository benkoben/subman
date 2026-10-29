package plan

import (
	"fmt"

	"github.com/Rhymond/go-money"
)

type BillingFrequency string

const (
	Monthly  BillingFrequency = "monthly"
	Annually BillingFrequency = "annually"
)

type Plan struct {
	Name        string
	Vendor      string
	Description *string
	// All prices are saved as swedish kronor (SEK)
	Price            money.Money
	BillingFrequency BillingFrequency
}

func NewPlan(name, vendor string, description *string, price int64, bf BillingFrequency) Plan {
	var p Plan

	p.Name = name
	p.Vendor = vendor
	if description != nil {
		p.Description = description
	}

	planPrice := money.New(price, money.SEK)
	p.Price = *planPrice
	p.BillingFrequency = bf

	return p
}

func (p *Plan) SetNewPrice(amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("amount cannot be 0 or less")
	}

	p.Price = *money.New(amount, money.SEK)
	return nil
}
