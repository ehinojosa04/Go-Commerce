package Models

import "fmt"

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func (p *Product) String() string {
	return fmt.Sprintf("[%d] %s - $%.2f (stock: %d)", p.ID, p.Name, p.Price, p.Stock)
}
