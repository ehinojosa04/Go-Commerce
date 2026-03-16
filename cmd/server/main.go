package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	Models "tui/pkg/models"
)

type session struct {
	conn   net.Conn
	role   Models.Role
	userID string
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

	defer func() {
		if len(sess.cart) > 0 {
			for _, item := range sess.cart {
				store.ReleaseStock(item.ProductID, item.Quantity)
			}
		}
		conn.Close()
	}()

	reader := bufio.NewReader(conn)

	sess.sendLine("Welcome to Go-Commerce!")
	sess.sendLine("Please login with: LOGIN <admin|consumer>")
	sess.sendLine("---")

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Connection closed (user %v)\n", sess.userID)
			return
		}

		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		parts := strings.Fields(message)
		command := strings.ToUpper(parts[0])

		logMessage := message
		if command == "LOGIN" || command == "REGISTER" {
			if len(parts) >= 3 {
				partsCopy := make([]string, len(parts))
				copy(partsCopy, parts)
				partsCopy[2] = "****" // Mask the password
				logMessage = strings.Join(partsCopy, " ")
			}
		}

		userDisplay := sess.userID
		if userDisplay == "" {
			userDisplay = "unauthenticated"
		}

		log.Printf("[IP: %s | User: %s | Role: %s] %s", conn.RemoteAddr(), userDisplay, sess.role, logMessage)

		if !sess.authed {
			if command != "LOGIN" && command != "REGISTER" {
				sess.sendLine("ERROR You must login first. Use: LOGIN <admin|consumer>")
				continue
			}
			handleAuth(sess, parts, store)
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
				for _, item := range sess.cart {
					store.ReleaseStock(item.ProductID, item.Quantity)
				}
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

func handleAuth(sess *session, parts []string, store *Models.Store) {
	if len(parts) < 3 {
		sess.sendLine("ERROR Usage: REGISTER <username> <pass> <role> OR LOGIN <username> <pass>")
		return
	}

	command := strings.ToUpper(parts[0])
	username := parts[1]
	password := parts[2]

	switch command {
	case "REGISTER":
		if len(parts) < 4 {
			sess.sendLine("ERROR Usage: REGISTER <username> <pass> <admin|consumer>")
			return
		}

		role := Models.Role(strings.ToLower(parts[3]))
		newUser := Models.NewUser(username, password, role)

		if err := store.AddUser(*newUser); err != nil {
			sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
			return
		}

		sess.role = newUser.Role
		sess.userID = newUser.UserID
		sess.authed = true

		sess.sendLine(fmt.Sprintf("OK Registered in user %v as %v", sess.userID, sess.role))
		sess.sendLine("Type HELP to see available commands.")

	case "LOGIN":
		storedUser, err := store.ValidateUser(username, password)
		if err != nil {
			sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
			return
		}

		sess.role = storedUser.Role
		sess.userID = storedUser.UserID
		sess.authed = true

		sess.sendLine(fmt.Sprintf("OK Logged in as %v user %v", sess.role, sess.userID))
		sess.sendLine("Type HELP to see available commands.")
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

	if err := store.ReserveStock(productID, qty); err != nil {
		sess.sendLine(fmt.Sprintf("ERROR %s", err.Error()))
		return
	}

	for i, item := range sess.cart {
		if item.ProductID == productID {
			sess.cart[i].Quantity += qty
			sess.sendLine(fmt.Sprintf("OK Updated cart (total qty: %d).", sess.cart[i].Quantity))
			return
		}
	}

	sess.cart = append(sess.cart, Models.OrderItem{ProductID: productID, Quantity: qty})
	sess.sendLine(fmt.Sprintf("OK Added x%d to cart.", qty))
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
	logFile, err := os.OpenFile("interactions.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Errorf("failed to open log file: %v", err))
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)

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
		log.Println("New connection from:", conn.RemoteAddr())
		go handleConnection(conn)
	}
}
