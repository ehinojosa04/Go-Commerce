package Models

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
)

type Store struct {
	Products map[int]*Product
	Orders   map[int]*Order
	Mu       sync.RWMutex
}

var (
	instance *Store
	once     sync.Once
)

func GetStore() *Store {
	once.Do(func() {
		instance = &Store{
			Products: make(map[int]*Product),
			Orders:   make(map[int]*Order),
		}
	})
	return instance
}

type Report struct {
}

func (s *Store) AddProduct(p Product) error {
	if _, alreadyExists := s.Products[p.ID]; alreadyExists {
		return errors.New("Product already exists (Edit it instead)")
	}
	s.Products[p.ID] = &p
	s.updateStore()
	return nil
}

func (s *Store) GetProduct(productID int) (*Product, error) {
	if product, exists := s.Products[productID]; exists {
		return product, nil
	} else {
		return nil, errors.New("Product not found")
	}
}

func (s *Store) GetProducts() []Product {
	var products []Product
	fmt.Println("Getting products...")
	for _, product := range s.Products {
		fmt.Println("Product: ", product.Name, "Price: ", product.Price, "Stock: ", product.Stock)
		products = append(products, *product)
	}
	return products
}

func (s *Store) GetProductsString() string {
	var result string

	for _, product := range s.Products {
		result += fmt.Sprintf(
			"PRODUCT %d %s %.2f %d\n",
			product.ID,
			product.Name,
			product.Price,
			product.Stock,
		)
	}

	return result
}

func (s *Store) UpdateStock(productID int, quantity int) error {
	if quantity <= 0 {
		return errors.New("Quantity must be greater than 0")
	}
	product := s.Products[productID]
	if product == nil {
		return errors.New("Product not found (Add it first)")
	} else {
		product.Stock = quantity
	}
	s.updateStore()
	return nil
}

func (s *Store) CreateOrder(items []OrderItem) (*Order, error) {
	fmt.Println("Creating order...")
	if items == nil {
		return nil, errors.New("Order must have at least one item")
	}
	orderID := 0
	for orderID == 0 || s.Orders[orderID] != nil {
		orderID = rand.Int()
	}

	orderID = rand.Int()

	store := GetStore()
	store.Mu.Lock()
	defer store.Mu.Unlock()
	storeProducts := store.Products

	o := newOrder(orderID, items)
	err := o.CalculateTotal(storeProducts)
	if err != nil {
		fmt.Println("Error calculating total: ", err)
		return nil, err
	}
	store.Orders[orderID] = o

	store.updateStore()
	fmt.Println("Order completed: The ID for you order is: ", orderID)
	return o, nil
}

func (s *Store) CompleteOrder(orderID int) error {
	if order, exists := s.Orders[orderID]; exists {
		order.Status = Completed
		s.updateStore()
		fmt.Println("Order completed: ", orderID)
		return nil
	} else {
		return errors.New("Order not found")
	}
}

func (s *Store) CancelOrder(orderID int) error {
	if order, exists := s.Orders[orderID]; exists {
		order.Status = Cancelled
		s.updateStore()
		fmt.Println("Order cancelled: ", orderID)
		return nil
	} else {
		return errors.New("Order not found")
	}
}

func (s *Store) SalesReport() {
	fmt.Println("----------- SALES REPORT -----------")
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	var totalSales float64 = 0
	for _, order := range s.Orders {
		if order.Status == Completed {
			fmt.Println("Order ID: ", order.ID, "Total: ", order.Total)
			totalSales += order.Total
		}
	}
	fmt.Println("------------------------------------")
}

func (s *Store) updateStore() {
	store := GetStore()
	store.Mu.Lock()
	defer store.Mu.Unlock()

	byteFile, err := json.MarshalIndent(store, "", " ")

	if err != nil {
		log.Panicf("Could not marshal: %v", err)
	}
	err = os.WriteFile("orders.json", byteFile, 0644)
	if err != nil {
		log.Panicf("Could not write file: %v", err)
	}
}

func readJsonFile() {
	var byteFile, errFile = os.ReadFile("orders.json")
	if errFile != nil {
		log.Panicf("Could not read JSON file: %v", errFile)
	}
	store := GetStore()
	store.Mu.Lock()
	defer store.Mu.Unlock()
	err := json.Unmarshal(byteFile, &store)
	if err != nil {
		panic(err)
	}
}

func main() {
	file, err := os.Open("orders.json")
	if err != nil {
		fmt.Println("Error while creating file: ", err)
		fmt.Println("Creating new file...")
		_, CreateErr := os.Create("contacts.json")
		if CreateErr != nil {
			fmt.Println("Error while creating file: ", CreateErr)
			return
		}

	}
	if err == nil {
		fmt.Println("Loading data from existing file...")
		readJsonFile()
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Error while closing file: ", err)
		}
	}(file)
}
