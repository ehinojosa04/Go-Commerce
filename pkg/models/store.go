package Models

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
)

type Store struct {
	Products map[int]*Product `json:"products"`
	Orders   map[int]*Order   `json:"orders"`
	Users    map[string]*User `json:"users"`
	Mu       sync.RWMutex     `json:"-"`
}

var (
	instance *Store
	once     sync.Once
	dataPath = "store_data.json"
)

func GetStore() *Store {
	once.Do(func() {
		instance = &Store{
			Products: make(map[int]*Product),
			Orders:   make(map[int]*Order),
			Users:    make(map[string]*User),
		}
		instance.loadFromDisk()
	})
	return instance
}

func (s *Store) AddProduct(p Product) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, alreadyExists := s.Products[p.ID]; alreadyExists {
		return errors.New("product already exists")
	}
	s.Products[p.ID] = &p
	s.saveToDisk()
	return nil
}

func (s *Store) DeleteProduct(productID int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, exists := s.Products[productID]; !exists {
		return errors.New("product not found")
	}
	delete(s.Products, productID)
	s.saveToDisk()
	return nil
}

func (s *Store) GetProduct(productID int) (*Product, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	if product, exists := s.Products[productID]; exists {
		return product, nil
	}
	return nil, errors.New("product not found")
}

func (s *Store) GetProducts() []Product {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	products := make([]Product, 0, len(s.Products))
	for _, product := range s.Products {
		products = append(products, *product)
	}
	return products
}

func (s *Store) GetProductsString() string {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	var result string
	for _, product := range s.Products {
		result += fmt.Sprintf(
			"  [%d] %s - $%.2f (stock: %d)\n",
			product.ID,
			product.Name,
			product.Price,
			product.Stock,
		)
	}
	if result == "" {
		return "  No products available.\n"
	}
	return result
}

func (s *Store) UpdateStock(productID int, quantity int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if quantity < 0 {
		return errors.New("quantity cannot be negative")
	}
	product, exists := s.Products[productID]
	if !exists {
		return errors.New("product not found")
	}
	product.Stock += quantity
	s.saveToDisk()
	return nil
}

func (s *Store) UpdatePrice(productID int, newPrice float64) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if newPrice < 0 {
		return errors.New("price cannot be negative")
	}
	product, exists := s.Products[productID]
	if !exists {
		return errors.New("product not found")
	}
	product.Price = newPrice
	s.saveToDisk()
	return nil
}

func (s *Store) CreateOrder(userID string, items []OrderItem) (*Order, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if len(items) == 0 {
		return nil, errors.New("order must have at least one item")
	}

	orderID := 0
	for orderID == 0 || s.Orders[orderID] != nil {
		orderID = rand.Intn(999999) + 1
	}

	order := newOrder(orderID, userID, items)
	if err := order.CalculateTotal(s.Products); err != nil {
		return nil, err
	}

	s.Orders[orderID] = order
	s.saveToDisk()
	return order, nil
}

func (s *Store) CompleteOrder(orderID int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	order, exists := s.Orders[orderID]
	if !exists {
		return errors.New("order not found")
	}
	order.Status = Completed
	s.saveToDisk()
	return nil
}

func (s *Store) CancelOrder(orderID int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	order, exists := s.Orders[orderID]
	if !exists {
		return errors.New("order not found")
	}
	order.Status = Cancelled

	for _, item := range order.Items {
		if product, ok := s.Products[item.ProductID]; ok {
			product.Stock += item.Quantity
		}
	}

	s.saveToDisk()
	return nil
}

func (s *Store) PurchaseHistory() string {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	if len(s.Orders) == 0 {
		return "  No orders found.\n"
	}

	var result string
	var totalRevenue float64
	for _, order := range s.Orders {
		if order == nil {
			continue
		}
		result += fmt.Sprintf("  Order #%d | User: %v | Status: %s | Total: $%.2f\n",
			order.ID, order.UserID, order.Status, order.Total)
		for _, item := range order.Items {
			name := fmt.Sprintf("Product#%d", item.ProductID)
			if p, exists := s.Products[item.ProductID]; exists {
				name = p.Name
			}
			result += fmt.Sprintf("    - %s x%d\n", name, item.Quantity)
		}
		if strings.EqualFold(string(order.Status), string(Completed)) {
			// Use stored order.Total so revenue matches the per-order totals we display
			// (item-based sum can be 0 if products were deleted or Items nil after JSON load)
			totalRevenue += order.Total
		}
	}
	result += fmt.Sprintf("  --- Total Revenue (completed): $%.2f ---\n", totalRevenue)
	return result
}

func (s *Store) UserOrderHistory(userID string) string {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	var result string
	found := false
	for _, order := range s.Orders {
		if order.UserID != userID {
			continue
		}
		found = true
		result += fmt.Sprintf("  Order #%d | Status: %s | Total: $%.2f\n",
			order.ID, order.Status, order.Total)
		for _, item := range order.Items {
			name := fmt.Sprintf("Product#%d", item.ProductID)
			if p, exists := s.Products[item.ProductID]; exists {
				name = p.Name
			}
			result += fmt.Sprintf("    - %s x%d\n", name, item.Quantity)
		}
	}
	if !found {
		return "  No orders found.\n"
	}
	return result
}

func (s *Store) saveToDisk() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal store: %v", err)
		return
	}
	if err := os.WriteFile(dataPath, data, 0644); err != nil {
		log.Printf("Failed to write store data: %v", err)
	}
}

func (s *Store) loadFromDisk() {
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, s); err != nil {
		log.Printf("Failed to unmarshal store data: %v", err)
	}
	if s.Products == nil {
		s.Products = make(map[int]*Product)
	}
	if s.Orders == nil {
		s.Orders = make(map[int]*Order)
	}
}

func (s *Store) AddUser(u User) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, alreadyExists := s.Users[u.UserID]; alreadyExists {
		return errors.New("user already exists")
	}

	s.Users[u.UserID] = &u
	s.saveToDisk()
	return nil
}

func (s *Store) ValidateUser(username string, password string) (*User, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	user, userExists := s.Users[username]
	if !userExists || user.Password != password {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *Store) ReserveStock(productID int, quantity int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	product, exists := s.Products[productID]
	if !exists {
		return errors.New("product not found")
	}
	if product.Stock < quantity {
		return fmt.Errorf("insufficient stock for product %d", productID)
	}
	product.Stock -= quantity
	s.saveToDisk()
	return nil
}

func (s *Store) ReleaseStock(productID int, quantity int) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	product, exists := s.Products[productID]
	if !exists {
		return nil
	}
	product.Stock += quantity
	s.saveToDisk()
	return nil
}
