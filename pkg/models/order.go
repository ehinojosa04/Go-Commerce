package Models

import (
	"errors"
	"fmt"
)

type OrderStatus string

const (
	Created   OrderStatus = "CREATED"
	Completed OrderStatus = "COMPLETED"
	Cancelled OrderStatus = "CANCELLED"
)

type Order struct {
	ID     int         `json:"id"`
	UserID int         `json:"user_id"`
	Items  []OrderItem `json:"items"`
	Total  float64     `json:"total"`
	Status OrderStatus `json:"status"`
}

func (o *Order) CalculateTotal(products map[int]*Product) error {
	total := 0.0
	for _, item := range o.Items {
		product := products[item.ProductID]
		if product == nil {
			return errors.New("product not found")
		}
		total += product.Price * float64(item.Quantity)
	}
	o.Total = total
	return nil
}

func newOrder(id int, userID int, items []OrderItem) *Order {
	return &Order{
		ID:     id,
		UserID: userID,
		Items:  items,
		Total:  0,
		Status: Created,
	}
}

func (o *Order) String() string {
	result := fmt.Sprintf("Order #%d | Status: %s | Total: $%.2f\n", o.ID, o.Status, o.Total)
	for _, item := range o.Items {
		result += fmt.Sprintf("  - Product %d x%d\n", item.ProductID, item.Quantity)
	}
	return result
}
