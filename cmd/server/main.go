package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	Models "tui/pkg/models"
)

type session struct {
	conn   net.Conn
	role   Models.Role
	userID int
	cart   []Models.OrderItem
	authed bool
}

func (s *session) send(msg string) {
	s.conn.Write([]byte(msg))
}

func (s *session) sendLine(msg string) {
	s.conn.Write([]byte(msg + "\n"))
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	sess := &session{
		conn:   conn,
		cart:   make([]Models.OrderItem, 0),
		authed: false,
	}

	store := Models.GetStore()
	reader := bufio.NewReader(conn)

	sess.sendLine("Welcome to Go-Commerce!")
	sess.sendLine("Please login with: LOGIN <admin|consumer>")
	sess.sendLine("---")

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Connection closed (user %d)\n", sess.userID)
			return
		}

		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		parts := strings.Fields(message)
		command := strings.ToUpper(parts[0])

		if !sess.authed {
			if command != "LOGIN" {
				sess.sendLine("ERROR You must login first. Use: LOGIN <admin|consumer>")
				continue
			}
			handleLogin(sess, parts)
			continue
		}

		switch command {
		case "HELP":
			handleHelp(sess)

		case "LIST_PRODUCTS":
			products := store.GetProductsString()
			sess.sendLine("--- Products ---")
			sess.send(products)
			sess.sendLine("----------------")

		case "ADD_PRODUCT":
			requireAdmin(sess, func() { handleAddProduct(sess, store, parts) })

		case "DELETE_PRODUCT":
			requireAdmin(sess, func() { handleDeleteProduct(sess, store, parts) })

		case "RESTOCK":
			requireAdmin(sess, func() { handleRestock(sess, store, parts) })

		case "UPDATE_PRICE":
			requireAdmin(sess, func() { handleUpdatePrice(sess, store, parts) })

		case "PURCHASE_HISTORY":
			requireAdmin(sess, func() {
				sess.sendLine("--- Purchase History ---")
				sess.send(store.PurchaseHistory())
				sess.sendLine("------------------------")
			})

		case "ADD_TO_CART":
			requireConsumer(sess, func() { handleAddToCart(sess, store, parts) })

		case "VIEW_CART":
			requireConsumer(sess, func() { handleViewCart(sess, store) })

		case "CLEAR_CART":
			requireConsumer(sess, func() {
				sess.cart = make([]Models.OrderItem, 0)
				sess.sendLine("OK Cart cleared.")
			})

		case "CHECKOUT":
			requireConsumer(sess, func() { handleCheckout(sess, store) })

		case "MY_ORDERS":
			requireConsumer(sess, func() {
				sess.sendLine("--- Your Orders ---")
				sess.send(store.UserOrderHistory(sess.userID))
				sess.sendLine("-------------------")
			})

		case "QUIT", "EXIT":
			sess.sendLine("Goodbye!")
			return

		default:
			sess.sendLine("ERROR Unknown command. Type HELP for available commands.")
		}
	}
}

func handleLogin(sess *session, parts []string) {
	if len(parts) < 2 {
		sess.sendLine("ERROR Usage: LOGIN <admin|consumer>")
		return
	}

	role := strings.ToLower(parts[1])
	switch Models.Role(role) {
	case Models.Admin:
		sess.role = Models.Admin
		sess.userID = rand.Intn(9000) + 1000
		sess.authed = true
		sess.sendLine(fmt.Sprintf("OK Logged in as admin (ID: %d)", sess.userID))
		sess.sendLine("Type HELP to see available commands.")
	case Models.Consumer:
		sess.role = Models.Consumer
		sess.userID = rand.Intn(9000) + 1000
		sess.authed = true
		sess.sendLine(fmt.Sprintf("OK Logged in as consumer (ID: %d)", sess.userID))
		sess.sendLine("Type HELP to see available commands.")
	default:
		sess.sendLine("ERROR Invalid role. Use: LOGIN <admin|consumer>")
	}
}

func handleHelp(sess *session) {
	sess.sendLine("--- Available Commands ---")
	sess.sendLine("  LIST_PRODUCTS              - List all products")
	sess.sendLine("  HELP                       - Show this help message")
	sess.sendLine("  QUIT                       - Disconnect")

	if sess.role == Models.Admin {
		sess.sendLine("")
		sess.sendLine("  Admin Commands:")
		sess.sendLine("  ADD_PRODUCT <name> <price> <stock>  - Add a new product")
		sess.sendLine("  DELETE_PRODUCT <id>                  - Delete a product")
		sess.sendLine("  RESTOCK <id> <quantity>              - Add stock to a product")
		sess.sendLine("  UPDATE_PRICE <id> <new_price>        - Update product price")
		sess.sendLine("  PURCHASE_HISTORY                     - View all order history")
	}

	if sess.role == Models.Consumer {
		sess.sendLine("")
		sess.sendLine("  Consumer Commands:")
		sess.sendLine("  ADD_TO_CART <product_id> <quantity>  - Add item to cart")
		sess.sendLine("  VIEW_CART                            - View your cart")
		sess.sendLine("  CLEAR_CART                           - Clear your cart")
		sess.sendLine("  CHECKOUT                             - Place order from cart")
		sess.sendLine("  MY_ORDERS                            - View your order history")
	}

	sess.sendLine("--------------------------")
}

func requireAdmin(sess *session, fn func()) {
	if sess.role != Models.Admin {
		sess.sendLine("ERROR Permission denied. Admin access required.")
		return
	}
	fn()
}

func requireConsumer(sess *session, fn func()) {
	if sess.role != Models.Consumer {
		sess.sendLine("ERROR Permission denied. Consumer access required.")
		return
	}
	fn()
}

func handleAddProduct(sess *session, store *Models.Store, parts []string) {
	if len(parts) < 4 {
		sess.sendLine("ERROR Usage: ADD_PRODUCT <name> <price> <stock>")
		return
	}

	name := parts[1]
	price, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		sess.sendLine("ERROR Invalid price.")
		return
	}
	stock, err := strconv.Atoi(parts[3])
	if err != nil {
		sess.sendLine("ERROR Invalid stock quantity.")
		return
	}

	id := rand.Intn(99999) + 1
	product := Models.Product{ID: id, Name: name, Price: price, Stock: stock}
	if err := store.AddProduct(product); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}
	sess.sendLine(fmt.Sprintf("OK Product added: [%d] %s - $%.2f (stock: %d)", id, name, price, stock))
}

func handleDeleteProduct(sess *session, store *Models.Store, parts []string) {
	if len(parts) < 2 {
		sess.sendLine("ERROR Usage: DELETE_PRODUCT <id>")
		return
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.sendLine("ERROR Invalid product ID.")
		return
	}

	if err := store.DeleteProduct(id); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}
	sess.sendLine(fmt.Sprintf("OK Product %d deleted.", id))
}

func handleRestock(sess *session, store *Models.Store, parts []string) {
	if len(parts) < 3 {
		sess.sendLine("ERROR Usage: RESTOCK <id> <quantity>")
		return
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.sendLine("ERROR Invalid product ID.")
		return
	}

	qty, err := strconv.Atoi(parts[2])
	if err != nil {
		sess.sendLine("ERROR Invalid quantity.")
		return
	}

	if err := store.UpdateStock(id, qty); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}
	sess.sendLine(fmt.Sprintf("OK Product %d restocked with %d units.", id, qty))
}

func handleUpdatePrice(sess *session, store *Models.Store, parts []string) {
	if len(parts) < 3 {
		sess.sendLine("ERROR Usage: UPDATE_PRICE <id> <new_price>")
		return
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.sendLine("ERROR Invalid product ID.")
		return
	}

	price, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		sess.sendLine("ERROR Invalid price.")
		return
	}

	if err := store.UpdatePrice(id, price); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}
	sess.sendLine(fmt.Sprintf("OK Product %d price updated to $%.2f.", id, price))
}

func handleAddToCart(sess *session, store *Models.Store, parts []string) {
	if len(parts) < 3 {
		sess.sendLine("ERROR Usage: ADD_TO_CART <product_id> <quantity>")
		return
	}

	productID, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.sendLine("ERROR Invalid product ID.")
		return
	}

	qty, err := strconv.Atoi(parts[2])
	if err != nil || qty <= 0 {
		sess.sendLine("ERROR Invalid quantity. Must be a positive number.")
		return
	}

	product, err := store.GetProduct(productID)
	if err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}

	if product.Stock < qty {
		sess.sendLine(fmt.Sprintf("ERROR Insufficient stock. Available: %d", product.Stock))
		return
	}

	for i, item := range sess.cart {
		if item.ProductID == productID {
			sess.cart[i].Quantity += qty
			sess.sendLine(fmt.Sprintf("OK Updated %s in cart (total qty: %d).", product.Name, sess.cart[i].Quantity))
			return
		}
	}

	sess.cart = append(sess.cart, Models.OrderItem{ProductID: productID, Quantity: qty})
	sess.sendLine(fmt.Sprintf("OK Added %s x%d to cart.", product.Name, qty))
}

func handleViewCart(sess *session, store *Models.Store) {
	if len(sess.cart) == 0 {
		sess.sendLine("Your cart is empty.")
		return
	}

	sess.sendLine("--- Your Cart ---")
	var total float64
	for _, item := range sess.cart {
		product, err := store.GetProduct(item.ProductID)
		if err != nil {
			sess.sendLine(fmt.Sprintf("  - Product %d (removed from store) x%d", item.ProductID, item.Quantity))
			continue
		}
		subtotal := product.Price * float64(item.Quantity)
		total += subtotal
		sess.sendLine(fmt.Sprintf("  - %s x%d @ $%.2f = $%.2f", product.Name, item.Quantity, product.Price, subtotal))
	}
	sess.sendLine(fmt.Sprintf("  Cart Total: $%.2f", total))
	sess.sendLine("-----------------")
}

func handleCheckout(sess *session, store *Models.Store) {
	if len(sess.cart) == 0 {
		sess.sendLine("ERROR Cart is empty. Add items before checkout.")
		return
	}

	order, err := store.CreateOrder(sess.userID, sess.cart)
	if err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}

	if err := store.CompleteOrder(order.ID); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR could not complete order: %s", err.Error()))
		return
	}

	sess.cart = make([]Models.OrderItem, 0)
	sess.sendLine(fmt.Sprintf("OK Order placed! Order #%d | Total: $%.2f", order.ID, order.Total))
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Go-Commerce TCP Server listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		fmt.Println("New connection from:", conn.RemoteAddr())
		go handleConnection(conn)
	}
}
