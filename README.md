# Go-Commerce

A concurrent client-server inventory and order processing engine built in Go, simulating an e-commerce platform similar to Amazon. The system supports multiple simultaneous clients, role-based access control, a terminal UI powered by Bubbletea/Charm, and persistent state saved to disk.

---

## Table of Contents

- [Project Description](#project-description)
- [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Clone the Repository](#clone-the-repository)
    - [Run the Server](#run-the-server)
    - [Connect a Client](#connect-a-client)
- [Administrative Functionalities](#administrative-functionalities)
- [Client Functionalities](#client-functionalities)
- [Features for Future Work](#features-for-future-work)

---

## Project Description

Go-Commerce is a TCP-based client-server application that simulates a real-world e-commerce backend. It consists of two main components:

**Product Management** — Administrators can add, remove, restock, and reprice products. The system prevents duplicate product IDs, rejects invalid inputs (negative prices, negative stock), and enforces alphanumeric product names.

**Order Management** — Consumers can browse the live product catalogue, build a shopping cart, and place orders. The server validates stock availability at checkout, deducts inventory atomically, calculates the order total, and tracks order status (`CREATED` → `COMPLETED` / `CANCELLED`).

All shared state is protected with a `sync.RWMutex`, so any number of clients can connect concurrently. Every mutation (product add/update/delete, order creation/completion) is immediately flushed to `store_data.json`, meaning the server can be restarted without losing data.

The client is a rich terminal UI (TUI) built with [Bubbletea v2](https://charm.land/bubbletea) and [Lipgloss v2](https://charm.land/lipgloss). It connects to the server over TCP and presents all commands as navigable menus — no manual command typing required.

---

## Getting Started

### Prerequisites

- **Go 1.22+** (the module uses Go 1.25 toolchain features)
- Internet access for the initial `go mod download` step (all dependencies are in `go.sum`)

### Clone the Repository

```bash
git clone https://github.com/ehinojosa04/Go-Commerce
cd Go-Commerce
go mod download
```

### Run the Server

Open a terminal and start the TCP server on port **8080**:

```bash
go run cmd/server/main.go
```

Expected output:

```
Go-Commerce TCP Server listening on port 8080
```

The server will print a line for every new connection and every disconnection. It loads `store_data.json` on startup if the file exists, so previous products and orders are restored automatically.

### Connect a Client

Open one or more **additional** terminals (one per simulated user) and run:

```bash
go run cmd/client/main.go
```

The TUI launches, connects to `localhost:8080`, and immediately presents a **role selection** screen. Use the arrow keys to highlight a role and press **Enter** to log in.

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate the menu |
| `Enter` | Select / submit |
| `/` | Filter menu items |
| `Esc` | Cancel current input |
| `ctrl+c` | Force quit |

> You can open as many client terminals as you like. All of them share the same live inventory — stock changes made by one client are immediately visible to all others.

---

## Administrative Functionalities

Log in as **Admin** to access the inventory management menu.

---

### Add Product

Select **"Add Product"** from the menu. An input field appears requesting:

```
Name Price Stock (space-separated):
```

**Example input:**

```
Laptop 999.99 50
```

**Server response:**

```
OK Product added: [42371] Laptop - $999.99 (stock: 50)
```

Business rules enforced:
- Name must be provided.
- Price must be a valid positive number; negative values are rejected.
- Stock must be a valid non-negative integer.
- Duplicate IDs are prevented — product IDs are generated randomly and checked against the existing map before insertion.

---

### Update Product Stock (Restock)

Select **"Restock Product"**. Input format:

```
Product ID and quantity:
```

**Example input:**

```
42371 25
```

**Server response:**

```
OK Product 42371 restocked with 25 units.
```

The server rejects negative restock quantities and returns an error if the product ID does not exist.

---

### Update Product Price

Select **"Update Price"**. Input format:

```
Product ID and new price:
```

**Example input:**

```
42371 849.99
```

**Server response:**

```
OK Product 42371 price updated to $849.99.
```

Prices must be non-negative; negative values are rejected with an error message.

---

### Delete Product

Select **"Delete Product"**. Input format:

```
Product ID:
```

**Example input:**

```
42371
```

**Server response:**

```
OK Product 42371 deleted.
```

---

### View Order History (Purchase History)

Select **"Purchase History"** to view all orders placed across all users, including a running total of revenue from completed orders.

**Example output:**

```
--- Purchase History ---
  Order #581234 | User: 3471 | Status: COMPLETED | Total: $1699.98
    - Laptop x2
  Order #904812 | User: 7823 | Status: COMPLETED | Total: $29.97
    - USB Cable x3
  --- Total Revenue (completed): $1729.95 ---
------------------------
```

---

## Client Functionalities

Log in as **Consumer** to access the shopping menu.

---

### List Products

Select **"List Products"** to see the live inventory.

**Example output:**

```
--- Products ---
  [42371] Laptop - $849.99 (stock: 48)
  [18293] USB Cable - $9.99 (stock: 97)
  [73841] Wireless Mouse - $29.99 (stock: 15)
----------------
```

Stock values reflect real-time availability. If another consumer has just bought all units of a product, it will show `stock: 0` on the next list.

---

### Add to Cart

Select **"Add to Cart"**. Input format:

```
Product ID and quantity:
```

**Example input:**

```
42371 1
```

**Server response:**

```
OK Added Laptop x1 to cart.
```

Adding the same product a second time merges quantities:

```
OK Updated Laptop in cart (total qty: 2).
```

Business rules enforced:
- Product must exist in the store.
- Requested quantity must be ≤ current stock; over-requesting is rejected at add-to-cart time.
- Quantity must be a positive integer.

---

### View Cart

Select **"View Cart"** to inspect current cart contents with per-item subtotals and a running total.

**Example output:**

```
--- Your Cart ---
  - Laptop x1 @ $849.99 = $849.99
  - USB Cable x3 @ $9.99 = $29.97
  Cart Total: $879.96
-----------------
```

---

### Clear Cart

Select **"Clear Cart"** to remove all items without placing an order.

**Server response:**

```
OK Cart cleared.
```

---

### Checkout (Place Order)

Select **"Checkout"** to convert the current cart into a confirmed order. The server:

1. Re-validates that sufficient stock exists for every cart item.
2. Deducts inventory atomically.
3. Calculates the order total.
4. Marks the order as `COMPLETED`.
5. Clears the client's cart.

**Example output:**

```
OK Order placed! Order #581234 | Total: $879.96
```

Business rules enforced:
- Cart must not be empty.
- Each product must still exist at checkout time.
- Stock must still cover the requested quantity (another client may have bought the last units between add-to-cart and checkout).
- An order cannot be completed twice.

---

### My Orders

Select **"My Orders"** to view the order history for the current session's user ID.

**Example output:**

```
--- Your Orders ---
  Order #581234 | Status: COMPLETED | Total: $879.96
    - Laptop x1
    - USB Cable x3
-------------------
```

---

## Features for Future Work

1. **User Registration & Authentication**
   Persist user accounts (username + hashed password) so that consumers can log out and reconnect later while retaining their order history under the same identity. Currently each session is assigned a random ephemeral user ID.

2. **Order Cancellation for Consumers**
   Allow consumers to cancel a `CREATED` or `COMPLETED` order within a defined window. Cancellation would restore the deducted stock and mark the order as `CANCELLED`, with the business rule that a cancelled order cannot be re-completed.

3. **Product Search & Filtering**
   Add `SEARCH_PRODUCTS <keyword>` and `FILTER_PRODUCTS <min_price> <max_price>` commands so consumers with large catalogues can quickly find what they need without scrolling through every item.

4. **Admin Audit Log**
   Write every administrative action (product added, price changed, stock updated, product deleted) to a structured log file with a timestamp, admin user ID, and the before/after values. This would satisfy audit and compliance requirements and complement the existing `store_data.json` persistence.