package models

import (
	"errors"
	"fmt"
)

type Order struct {
	ID     int
	Items  []OrderItem
	Total  float64
	Status OrderStatus
}

type OrderStatus string

func (o *Order) CalculateTotal(products map[int]*Product) error {
	total := 0.0

	for _, item := range o.Items {
		product := products[item.ProductID]
		if product == nil {
			return errors.New("Product not found")
		}
		total += product.Price * float64(item.Quantity)
	}
	o.Total = total
	return nil
}

const (
	Created   OrderStatus = "CREATED"
	Completed OrderStatus = "COMPLETED"
	Cancelled OrderStatus = "CANCELLED"
)

func newOrder(id int, items []OrderItem) *Order {
	total := 0.0
	status := Created
	return &Order{ID: id, Items: items, Total: total, Status: status}
}

func (o *Order) listOrderItems() {
	fmt.Println("Products in order/cart:")
	for _, product := range o.Items {
		fmt.Println("Product: ", product.ProductID, "Quantity: ", product.Quantity)
	}
	fmt.Println("Order Total: ", o.Total, "Order Status: ", o.Status, "")
}

func main() {

}
