package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const serverAddr = "localhost:8080"

// ---- styles ----

type styles struct {
	app         lipgloss.Style
	title       lipgloss.Style
	status      lipgloss.Style
	outputBox   lipgloss.Style
	outputTitle lipgloss.Style
	errorTitle  lipgloss.Style
	roleTitle   lipgloss.Style
}

func newStyles(darkBG bool) styles {
	lightDark := lipgloss.LightDark(darkBG)
	return styles{
		app: lipgloss.NewStyle().
			Padding(1, 2),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1),
		status: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#04B575"), lipgloss.Color("#04B575"))),
		outputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginTop(1),
		outputTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true),
		errorTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true),
		roleTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1),
	}
}

// ---- list item ----

type menuItem struct {
	title       string
	description string
	choice      string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.description }
func (i menuItem) FilterValue() string { return i.title }

// ---- messages ----

type connResultMsg struct {
	conn net.Conn
	err  error
}

type serverOutputMsg struct {
	output string
}

// connectionLostMsg is sent when a read fails.
type connectionLostMsg struct {
	output string
	err    error
}

type menuSelectedMsg struct {
	choice string
}

// ---- model ----

type state int

const (
	stateConnecting   state = iota
	stateAuthMenu           // Elegir entre Login o Register
	stateRoleSelect         // Elegir rol (solo para Register)
	stateAuthUsername       // Escribir usuario
	stateAuthPassword       // Escribir contraseña
	stateLoading
	stateLoadingQuit
	stateMenu
	stateInput
	stateAuthError
	stateViewingOutput
	stateConnectionFailed
	stateConnectionLost
	stateQuit
)

type model struct {
	styles          styles
	darkBG          bool
	width           int
	height          int
	conn            net.Conn
	role            string
	state           state
	list            list.Model
	delegate        list.DefaultDelegate
	keys            *delegateKeyMap
	serverOut       string
	connectionErr   string
	lastSentCommand string

	// Campos para el flujo de autenticación
	authAction   string // "LOGIN" o "REGISTER"
	authUsername string
	authPassword string

	input       textinput.Model
	inputPrompt string
	cmdPrefix   string
}

type delegateKeyMap struct {
	choose key.Binding
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
	}
}

func (m *model) writeConn(data []byte) bool {
	if m.conn == nil {
		return false
	}
	_, err := m.conn.Write(data)
	if err != nil {
		m.state = stateConnectionLost
		m.connectionErr = err.Error()
		_ = m.conn.Close()
		m.conn = nil
		return false
	}
	return true
}

func (m model) Init() tea.Cmd {
	return connectCmd()
}

func connectCmd() tea.Cmd {
	return func() tea.Msg {
		conn, err := net.Dial("tcp", serverAddr)
		return connResultMsg{conn: conn, err: err}
	}
}

func readServerCmd(conn net.Conn) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return serverOutputMsg{output: ""}
		}
		defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
		var out strings.Builder
		var readErr error
		r := bufio.NewReaderSize(conn, 4096)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			line, err := r.ReadString('\n')
			if err != nil {
				readErr = err
				break
			}
			out.WriteString(line)
			_ = conn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
			_, peekErr := r.Peek(1)
			if peekErr != nil {
				break
			}
		}
		if readErr != nil {
			return connectionLostMsg{output: out.String(), err: readErr}
		}
		return serverOutputMsg{output: out.String()}
	}
}

func viewOutputTitle(cmd string) string {
	switch cmd {
	case "LIST_PRODUCTS":
		return "Products"
	case "VIEW_CART":
		return "Cart"
	case "MY_ORDERS":
		return "My Orders"
	case "PURCHASE_HISTORY":
		return "Purchase History"
	case "HELP":
		return "Help"
	case "CHECKOUT":
		return "Checkout"
	case "CLEAR_CART":
		return "Cart cleared"
	case "ADD_PRODUCT", "DELETE_PRODUCT", "RESTOCK", "UPDATE_PRICE", "ADD_TO_CART":
		return "Result"
	default:
		return "Response"
	}
}

func (m *model) updateListSize() {
	if m.state != stateRoleSelect && m.state != stateMenu && m.state != stateAuthMenu {
		return
	}
	if m.width <= 0 || m.height <= 0 {
		return
	}
	h, v := m.styles.app.GetFrameSize()
	m.list.SetSize(m.width-h, m.height-v-6)
}

func authMenuItems() []list.Item {
	return []list.Item{
		menuItem{"Login", "Inicia sesión en tu cuenta", "LOGIN"},
		menuItem{"Register", "Crea una cuenta nueva", "REGISTER"},
		menuItem{"Quit", "Desconectar y salir", "QUIT"},
	}
}

func adminMenuItems() []list.Item {
	return []list.Item{
		menuItem{"List Products", "View all products in the store", "LIST_PRODUCTS"},
		menuItem{"Add Product", "Add a new product (name, price, stock)", "ADD_PRODUCT"},
		menuItem{"Delete Product", "Remove a product by ID", "DELETE_PRODUCT"},
		menuItem{"Restock Product", "Add stock to a product", "RESTOCK"},
		menuItem{"Update Price", "Change a product's price", "UPDATE_PRICE"},
		menuItem{"Purchase History", "View purchase history", "PURCHASE_HISTORY"},
		menuItem{"Help", "Show help", "HELP"},
		menuItem{"Quit", "Disconnect and exit", "QUIT"},
	}
}

func consumerMenuItems() []list.Item {
	return []list.Item{
		menuItem{"List Products", "Browse available products", "LIST_PRODUCTS"},
		menuItem{"Add to Cart", "Add product to cart (ID, quantity)", "ADD_TO_CART"},
		menuItem{"View Cart", "See your cart contents", "VIEW_CART"},
		menuItem{"Clear Cart", "Remove all items from cart", "CLEAR_CART"},
		menuItem{"Checkout", "Place order", "CHECKOUT"},
		menuItem{"My Orders", "View your order history", "MY_ORDERS"},
		menuItem{"Help", "Show help", "HELP"},
		menuItem{"Quit", "Disconnect and exit", "QUIT"},
	}
}

func roleItems() []list.Item {
	return []list.Item{
		menuItem{"Admin", "Manage products and view history", "admin"},
		menuItem{"Consumer", "Shop and place orders", "consumer"},
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		m.styles = newStyles(m.darkBG)
		if m.state == stateRoleSelect || m.state == stateMenu || m.state == stateAuthMenu {
			m.list.Styles.Title = m.styles.title
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateListSize()
		return m, nil

	case connResultMsg:
		if msg.err != nil {
			m.state = stateConnectionFailed
			m.connectionErr = msg.err.Error()
			return m, nil
		}
		m.conn = msg.conn
		m.state = stateLoading
		return m, tea.Batch(tea.RequestBackgroundColor, readServerCmd(m.conn))

	case connectionLostMsg:
		m.state = stateConnectionLost
		m.connectionErr = msg.err.Error()
		if msg.output != "" {
			m.serverOut = msg.output + "\n\n  --- connection closed ---"
		} else {
			m.serverOut = ""
		}
		if m.conn != nil {
			_ = m.conn.Close()
			m.conn = nil
		}
		return m, nil

	case serverOutputMsg:
		if m.state == stateLoading || m.state == stateLoadingQuit {
			m.serverOut = msg.output
			if m.state == stateLoadingQuit {
				return m, tea.Quit
			}

			// Si no hemos iniciado el flujo de autenticación, mostramos el menú inicial
			if m.authAction == "" {
				m.state = stateAuthMenu
				m.buildAuthList()
			} else if m.lastSentCommand != "" {
				m.state = stateViewingOutput
			} else {
				if strings.Contains(m.serverOut, "ERROR") {
					m.state = stateAuthError
				} else {
					if m.authAction == "LOGIN" {
						if strings.Contains(m.serverOut, "as admin") {
							m.role = "admin"
						} else {
							m.role = "consumer"
						}
					}
					m.state = stateMenu
					m.buildMenuList()
				}
			}
		}
		return m, nil

	case menuSelectedMsg:
		switch m.state {
		case stateAuthMenu:
			if msg.choice == "QUIT" {
				if !m.writeConn([]byte("QUIT\n")) {
					return m, nil
				}
				m.state = stateLoadingQuit
				return m, readServerCmd(m.conn)
			}

			m.authAction = msg.choice // Guarda si es "LOGIN" o "REGISTER"
			if m.authAction == "REGISTER" {
				m.state = stateRoleSelect
				m.buildRoleList()
			} else {
				// Si es LOGIN, saltamos directo a pedir el usuario
				m.state = stateAuthUsername
				m.input = textinput.New()
				m.input.Placeholder = "Tu nombre de usuario"
				m.input.Focus()
			}
			return m, nil

		case stateRoleSelect:
			m.role = msg.choice
			m.state = stateAuthUsername
			m.input = textinput.New()
			m.input.Placeholder = "Tu nombre de usuario"
			m.input.Focus()
			return m, nil

		case stateMenu:
			choice := msg.choice
			if choice == "QUIT" {
				if !m.writeConn([]byte("QUIT\n")) {
					return m, nil
				}
				m.state = stateLoadingQuit
				return m, readServerCmd(m.conn)
			}
			switch choice {
			case "ADD_PRODUCT":
				m.state = stateInput
				m.inputPrompt = "Name Price Stock (space-separated):"
				m.cmdPrefix = "ADD_PRODUCT "
				m.input = textinput.New()
				m.input.Placeholder = "e.g. Widget 9.99 100"
				m.input.Focus()
				return m, nil
			case "DELETE_PRODUCT":
				m.state = stateInput
				m.inputPrompt = "Product ID:"
				m.cmdPrefix = "DELETE_PRODUCT "
				m.input = textinput.New()
				m.input.Placeholder = "ID"
				m.input.Focus()
				return m, nil
			case "RESTOCK":
				m.state = stateInput
				m.inputPrompt = "Product ID and quantity:"
				m.cmdPrefix = "RESTOCK "
				m.input = textinput.New()
				m.input.Placeholder = "ID 10"
				m.input.Focus()
				return m, nil
			case "UPDATE_PRICE":
				m.state = stateInput
				m.inputPrompt = "Product ID and new price:"
				m.cmdPrefix = "UPDATE_PRICE "
				m.input = textinput.New()
				m.input.Placeholder = "ID 19.99"
				m.input.Focus()
				return m, nil
			case "ADD_TO_CART":
				m.state = stateInput
				m.inputPrompt = "Product ID and quantity:"
				m.cmdPrefix = "ADD_TO_CART "
				m.input = textinput.New()
				m.input.Placeholder = "ID 2"
				m.input.Focus()
				return m, nil
			}
			// Comandos que no requieren input extra
			m.lastSentCommand = choice
			cmd := choice + "\n"
			if !m.writeConn([]byte(cmd)) {
				return m, nil
			}
			m.state = stateLoading
			return m, readServerCmd(m.conn)
		}
	}

	// Auth error: let the user go back to the auth menu
	if m.state == stateAuthError {
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			m.authAction = ""
			m.authUsername = ""
			m.authPassword = ""
			m.serverOut = ""
			m.state = stateAuthMenu
			m.buildAuthList()
			return m, nil
		}
	}

	// Manejo de pantallas de error de conexión
	if m.state == stateConnectionFailed || m.state == stateConnectionLost {
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			return m, tea.Quit
		}
	}

	// Viendo la salida del servidor
	if m.state == stateViewingOutput {
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			m.state = stateMenu
			m.buildMenuList()
			return m, nil
		}
	}

	// Manejo de la captura del nombre de usuario
	if m.state == stateAuthUsername {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		if k, ok := msg.(tea.KeyPressMsg); ok {
			if k.String() == "enter" {
				val := strings.TrimSpace(m.input.Value())
				if val != "" {
					m.authUsername = val
					m.state = stateAuthPassword
					// Preparamos el input para la contraseña
					m.input = textinput.New()
					m.input.Placeholder = "Tu contraseña"
					m.input.EchoMode = textinput.EchoPassword
					m.input.EchoCharacter = '•'
					m.input.Focus()
				}
			} else if k.String() == "esc" || k.String() == "ctrl+c" {
				// Cancelar y volver al menú principal
				m.state = stateAuthMenu
				m.authAction = ""
				m.buildAuthList()
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Manejo de la captura de la contraseña y envío al servidor
	if m.state == stateAuthPassword {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		if k, ok := msg.(tea.KeyPressMsg); ok {
			if k.String() == "enter" {
				val := strings.TrimSpace(m.input.Value())
				if val != "" {
					m.authPassword = val
					var finalCmd string
					if m.authAction == "LOGIN" {
						finalCmd = fmt.Sprintf("LOGIN %s %s\n", m.authUsername, m.authPassword)
					} else { // REGISTER
						finalCmd = fmt.Sprintf("REGISTER %s %s %s\n", m.authUsername, m.authPassword, m.role)
					}

					if !m.writeConn([]byte(finalCmd)) {
						return m, nil
					}

					// Limpiamos todo y esperamos la respuesta del servidor
					m.state = stateLoading
					m.lastSentCommand = ""
					return m, readServerCmd(m.conn)
				}
			} else if k.String() == "esc" || k.String() == "ctrl+c" {
				m.state = stateAuthMenu
				m.authAction = ""
				m.buildAuthList()
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Manejo general de inputs (e.g. ADD_PRODUCT)
	if m.state == stateInput {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
			if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
				val := strings.TrimSpace(m.input.Value())
				if val != "" {
					m.lastSentCommand = strings.TrimSpace(strings.Fields(m.cmdPrefix)[0])
					fullCmd := m.cmdPrefix + val + "\n"
					if !m.writeConn([]byte(fullCmd)) {
						m.input.Reset()
						return m, tea.Batch(cmds...)
					}
					m.state = stateLoading
					m.input.Reset()
					return m, tea.Sequence(tea.Batch(cmds...), readServerCmd(m.conn))
				}
			}
			if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "esc" || k.String() == "ctrl+c") {
				m.state = stateMenu
				m.input.Reset()
				return m, tea.Batch(cmds...)
			}
			return m, tea.Batch(cmds...)
		}
	}

	// Actualización de las listas
	if m.state == stateAuthMenu || m.state == stateRoleSelect || m.state == stateMenu {
		newList, cmd := m.list.Update(msg)
		m.list = newList
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) buildAuthList() {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.UpdateFunc = func(msg tea.Msg, l *list.Model) tea.Cmd {
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.choose) {
			if i, ok := l.SelectedItem().(menuItem); ok {
				return func() tea.Msg { return menuSelectedMsg{i.choice} }
			}
		}
		return nil
	}
	delegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{m.keys.choose} }

	items := authMenuItems()
	l := list.New(items, delegate, 0, 0)
	l.Title = " Autenticación "
	l.Styles.Title = m.styles.title
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	m.list = l
	m.delegate = delegate
	m.updateListSize()
}

func (m *model) buildRoleList() {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.UpdateFunc = func(msg tea.Msg, l *list.Model) tea.Cmd {
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.choose) {
			if i, ok := l.SelectedItem().(menuItem); ok {
				return func() tea.Msg { return menuSelectedMsg{i.choice} }
			}
		}
		return nil
	}
	delegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{m.keys.choose} }
	items := roleItems()
	l := list.New(items, delegate, 0, 0)
	l.Title = " Select role "
	l.Styles.Title = m.styles.roleTitle
	l.SetShowStatusBar(false)
	l.SetShowFilter(true)
	l.SetFilteringEnabled(true)
	m.list = l
	m.delegate = delegate
	m.updateListSize()
}

func (m *model) buildMenuList() {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.UpdateFunc = func(msg tea.Msg, l *list.Model) tea.Cmd {
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.choose) {
			if i, ok := l.SelectedItem().(menuItem); ok {
				return func() tea.Msg { return menuSelectedMsg{i.choice} }
			}
		}
		return nil
	}
	delegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{m.keys.choose} }
	var items []list.Item
	if m.role == "admin" {
		items = adminMenuItems()
	} else {
		items = consumerMenuItems()
	}
	l := list.New(items, delegate, 0, 0)
	title := " Consumer menu "
	if m.role == "admin" {
		title = " Admin menu "
	}
	l.Title = title
	l.Styles.Title = m.styles.title
	l.SetShowStatusBar(true)
	l.SetShowFilter(true)
	l.SetFilteringEnabled(true)
	m.list = l
	m.delegate = delegate
	m.updateListSize()
}

func (m model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.styles.app.Render(""))

	switch m.state {
	case stateConnecting:
		b.WriteString(m.styles.title.Render(" Connecting… ") + "\n\n  " + serverAddr)
	case stateLoading, stateLoadingQuit:
		b.WriteString(m.styles.title.Render(" Loading… ") + "\n\n")
		if m.serverOut != "" {
			b.WriteString(m.styles.outputBox.Render(
				m.styles.outputTitle.Render(" Server response ") + "\n" + m.serverOut,
			))
		}
	case stateAuthMenu, stateRoleSelect, stateMenu:
		b.WriteString(m.list.View())
		if m.serverOut != "" {
			b.WriteString(m.styles.outputBox.Render(
				m.styles.outputTitle.Render(" Last response ") + "\n" + m.serverOut,
			))
		}
	case stateAuthUsername:
		b.WriteString(m.styles.title.Render(" Nombre de Usuario ") + "\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n  (enter=continuar, esc=cancelar)")
	case stateAuthPassword:
		b.WriteString(m.styles.title.Render(" Contraseña ") + "\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n  (enter=enviar, esc=cancelar)")
	case stateAuthError:
		b.WriteString(m.styles.outputBox.Render(
			m.styles.errorTitle.Render(" Authentication Failed ") + "\n\n  " + strings.TrimSpace(m.serverOut) + "\n\n  " + m.styles.status.Render("Press Enter to try again"),
		))
	case stateInput:
		b.WriteString(m.styles.title.Render(" "+m.inputPrompt+" ") + "\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n  (enter=submit, esc=cancel)")
	case stateViewingOutput:
		title := viewOutputTitle(m.lastSentCommand)
		b.WriteString(m.styles.outputBox.Render(
			m.styles.outputTitle.Render(" "+title+" ") + "\n\n" + m.serverOut + "\n\n  " + m.styles.status.Render("Press Enter to continue"),
		))
	case stateConnectionFailed:
		b.WriteString(m.styles.outputBox.Render(
			m.styles.outputTitle.Render(" Connection failed ") + "\n\n  " + m.connectionErr + "\n\n  " + m.styles.status.Render("Press Enter to exit"),
		))
	case stateConnectionLost:
		header := m.styles.outputTitle.Render(" Connection lost ") + "\n\n  " + m.connectionErr
		if m.serverOut != "" {
			header += "\n\n" + m.serverOut
		}
		b.WriteString(m.styles.outputBox.Render(header + "\n\n  " + m.styles.status.Render("Press Enter to exit")))
	case stateQuit:
		b.WriteString(m.serverOut)
	}
	v := tea.NewView(m.styles.app.Render(b.String()))
	v.AltScreen = true
	v.WindowTitle = "go-commerce"
	return v
}

func initialModel() model {
	m := model{}
	m.styles = newStyles(false)
	m.keys = newDelegateKeyMap()
	m.state = stateConnecting
	return m
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
