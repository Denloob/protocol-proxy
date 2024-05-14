package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Denloob/protocol-proxy/styles"
	"github.com/Denloob/protocol-proxy/symbols"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	boxer "github.com/treilik/bubbleboxer"
)

var symbolMap = symbols.NerdFontMap

type Args struct {
	inPort  int
	outPort int
	outIP   string
}

func getArgs() Args {
	inPortPtr := flag.Int("in-port", 0, "The in port on which to listen")
	outPortPtr := flag.Int("out-port", 0, "The out port to which to output")
	outIPPtr := flag.String("out-ip", "127.0.0.1", "The out ip to which to output")
	flag.Parse()

	if *inPortPtr == 0 || *outPortPtr == 0 {
		fmt.Printf("%v: Both -in-port and -out-port ports must be specified\n", strings.Join(os.Args, " "))
		fmt.Println("Run with -help for usage.")

		os.Exit(1)
	}

	return Args{*inPortPtr, *outPortPtr, *outIPPtr}
}

func forward(source io.Reader, dest io.Writer, handleTransmittion func([]byte) []byte) {
	for {
		buffer := make([]byte, 1<<16)
		size, err := source.Read(buffer)
		if err != nil {
			log.Fatalf("Read failed: %v", err)
		}

		go func() {
			newBuffer := handleTransmittion(buffer[:size])
			if newBuffer == nil { // Packet dropped, skip
				return
			}

			_, err := dest.Write(newBuffer)

			if err != nil {
				log.Printf("Write failed: %v", err)
			}
		}()
	}
}

func isCharacter(char byte) bool {
	return ' ' <= char && char <= '~'
}

const DEFAULT_EXTRACT_STRINGS_MIN_LENGTH = 4

func extractStrings(buffer []byte, minStringLength int) []string {
	var foundStrings []string

	var stringBegin int
	insideString := false

	for i, char := range buffer {
		if !isCharacter(char) {
			if insideString && i-stringBegin >= minStringLength {
				foundStrings = append(foundStrings, string(buffer[stringBegin:i]))
				insideString = false
			}
			continue
		}

		if !insideString {
			insideString = true
			stringBegin = i
		}
	}

	if insideString && len(buffer)-stringBegin >= minStringLength {
		foundStrings = append(foundStrings, string(buffer[stringBegin:]))
	}

	return foundStrings
}

type TransmittionDirection int

const (
	TRANSMITTION_DIRECTION_TO_SERVER TransmittionDirection = iota
	TRANSMITTION_DIRECTION_TO_CLIENT
)

func (direction TransmittionDirection) String() string {
	switch direction {
	case TRANSMITTION_DIRECTION_TO_SERVER:
		return symbolMap[symbols.ScArrowLeft]
	case TRANSMITTION_DIRECTION_TO_CLIENT:
		return symbolMap[symbols.ScArrowRight]
	default:
		panic("Invalid direction")
	}
}

type Status int

const (
	STATUS_PENDING Status = iota
	STATUS_TRANSMITED
	STATUS_DROPPED
)

func (status Status) String() string {
	switch status {
	case STATUS_PENDING:
		return symbolMap[symbols.ScClock]
	case STATUS_TRANSMITED:
		return symbolMap[symbols.ScSentMail]
	case STATUS_DROPPED:
		return symbolMap[symbols.ScTrashCan]
	default:
		panic("Invalid status")
	}
}

func NewTCPMessage(transmittionDirection TransmittionDirection, content []byte) *TCPMessage {
	return &TCPMessage{
		content:   content,
		edited:    false,
		status:    STATUS_PENDING,
		time:      time.Now(),
		direction: transmittionDirection,

		transmitChan: make(chan bool),
	}
}

func (proxy *Proxy) createTransmittionHandler(transmittionDirection TransmittionDirection) func(buffer []byte) []byte {
	return func(buffer []byte) []byte {
		message := NewTCPMessage(transmittionDirection, buffer)

		proxy.messages = append(proxy.messages, message)

		<-message.transmitChan

		return message.content
	}
}

func Must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

type Model struct {
	tui *boxer.Boxer
}

type TCPMessage struct {
	content   []byte
	edited    bool
	status    Status
	time      time.Time
	direction TransmittionDirection

	transmitChan chan bool
}

func (message *TCPMessage) Transmit() error {
	switch message.status {
	case STATUS_PENDING:
		message.status = STATUS_TRANSMITED

	case STATUS_TRANSMITED:
		return fmt.Errorf("The message was already transmitted. Can't retransmit.")
	case STATUS_DROPPED:
		return fmt.Errorf("The message was dropped. Can't transmit.")
	default:
		return fmt.Errorf("The message cannot be transmitted.")
	}

	message.transmitChan <- true

	return nil
}

func (message TCPMessage) SetContent(newContent []byte) error {
	if message.status != STATUS_PENDING {
		return fmt.Errorf("The message can no longer be edited.")
	}

	message.content = newContent
	message.edited = true

	return nil
}

func (message TCPMessage) Content() []byte {
	return message.content
}

func (message TCPMessage) String() string {
	messageState := message.status.String()
	if message.edited {
		messageState += " " + symbolMap[symbols.ScPen]
	}

	return fmt.Sprintf("[%v] %v %v (%v bytes)", message.time.Format(time.TimeOnly), messageState, message.direction, len(message.content))
}

type Proxy struct {
	args                 Args
	messages             []*TCPMessage
	vieweingMessage      bool
	selectedMessageIndex int
}

type TickMsg time.Time

func Tick() tea.Msg {
	return TickMsg(time.Now())
}

func NewProxy(args Args) *Proxy {
	return &Proxy{
		args:                 args,
		messages:             nil,
		vieweingMessage:      false,
		selectedMessageIndex: -1,
	}
}

func (p *Proxy) tick() tea.Cmd {
	if len(p.messages) > 0 && p.selectedMessageIndex == -1 {
		p.selectedMessageIndex = 0

		return CreateViewMsgCmd(p.messages[p.selectedMessageIndex])
	}

	return nil
}

func (p *Proxy) Init() tea.Cmd {
	return func() tea.Msg {
		go p.Run()
		return Tick()
	}
}
func (p *Proxy) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds tea.BatchMsg

	switch msg := msg.(type) {
	case tea.KeyMsg:
		newProxy, cmd := keyMap.Handle(p, msg)
		p = newProxy.(*Proxy)
		cmds = append(cmds, cmd)
	case TickMsg:
		cmds = append(cmds, Tick)
		cmds = append(cmds, p.tick())
	case editBufferInEditorMsg:
		if msg.err != nil {
			log.Printf("error during message editing: %v\n", msg.err)
		} else {
			p.messages[p.selectedMessageIndex].content = msg.newBuffer
		}
	}

	return p, tea.Batch(cmds...)
}
func (p *Proxy) View() string {
	var res string
	for i, message := range p.messages {
		line := fmt.Sprintf("%d. %v", i+1, message)

		style := styles.Unstyled

		if p.vieweingMessage {
			if i == p.selectedMessageIndex {
				style = styles.UnfocusedSelected
			} else {
				style = styles.Unfocused
			}
		} else if i == p.selectedMessageIndex {
			style = styles.Selected
		}

		line = style.Render(line)
		res += line + "\n"
	}

	return res
}
func (proxy *Proxy) Run() {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", proxy.args.inPort))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func(inConn net.Conn) {
			var dialer net.Dialer

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			outConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", proxy.args.outIP, proxy.args.outPort))
			if err != nil {
				log.Fatalf("Failed to dial: %v", err)
			}

			go forward(inConn, outConn, proxy.createTransmittionHandler(TRANSMITTION_DIRECTION_TO_SERVER))
			go forward(outConn, inConn, proxy.createTransmittionHandler(TRANSMITTION_DIRECTION_TO_CLIENT))
		}(conn)
	}
}

type Console struct {
	*strings.Builder
	name string
}

type KeyMap interface {
	Handle(model tea.Model, keyMsg tea.KeyMsg) (tea.Model, tea.Cmd)
}

type MainKeyMap struct {
	Quit,
	View,
	Up,
	Down key.Binding
}

var mainKeymap = &MainKeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp(symbolMap[symbols.ScArrowUp]+"/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp(symbolMap[symbols.ScArrowDown]+"/j", "move down"),
	),
	View: key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp(symbolMap[symbols.ScSpace]+"/"+symbolMap[symbols.ScEnter], "transmit"),
	),
}

func (k *MainKeyMap) Handle(model tea.Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	proxy := model.(*Proxy)
	selectedMessageChanged := false

	switch {
	case key.Matches(msg, k.Quit):
		return proxy, tea.Quit
	case key.Matches(msg, k.Up) && proxy.selectedMessageIndex > 0:
		proxy.selectedMessageIndex--
		selectedMessageChanged = true
	case key.Matches(msg, k.Down) && proxy.selectedMessageIndex < len(proxy.messages)-1:
		proxy.selectedMessageIndex++
		selectedMessageChanged = true
	case key.Matches(msg, k.View) && len(proxy.messages) > 0:
		proxy.vieweingMessage = true
		keyMap = messageViewKeymap
	}

	if selectedMessageChanged {
		return proxy, CreateViewMsgCmd(proxy.messages[proxy.selectedMessageIndex])
	}

	return proxy, nil
}

type editBufferInEditorMsg struct {
	newBuffer []byte
	err       error
}

func editBufferInEditor(buffer []byte) (tea.Cmd, error) {
	tempfile, err := os.CreateTemp("", "hexdump*.bin")
	if err != nil {
		return nil, err
	}

	filename := tempfile.Name()

	tempfile.Write(buffer)
	tempfile.Close()

	editor := os.Getenv("EDITOR")

	cmd := exec.Command(editor, filename)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tempfile.Name())

		newBuffer, err := os.ReadFile(filename)
		return editBufferInEditorMsg{newBuffer, err}
	}), nil
}

type ViewMessageKeyMap struct {
	Quit,
	ExitView,
	Edit key.Binding
}

var messageViewKeymap = &ViewMessageKeyMap{
	Quit: key.NewBinding(
		key.WithKeys("Q", "ctrl+c"),
		key.WithHelp("Q", "quit"),
	),
	ExitView: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "exit view"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
}

func (k ViewMessageKeyMap) Handle(model tea.Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	proxy := model.(*Proxy)

	switch {
	case key.Matches(msg, k.Quit):
		return proxy, tea.Quit
	case key.Matches(msg, k.ExitView):
		proxy.vieweingMessage = false
		keyMap = mainKeymap
	case key.Matches(msg, k.Edit):
		messageContent := proxy.messages[proxy.selectedMessageIndex].content
		cmd, err := editBufferInEditor(messageContent)
		if err != nil {
			log.Printf("Edit in editor error: %s\n", err)
			return proxy, nil
		}

		return proxy, cmd
	}

	return proxy, nil
}

var keyMap KeyMap = mainKeymap

type MessageViewModel struct {
	viewedMessage *TCPMessage

	proxy *Proxy
}

type ViewMessageMsg struct {
	message *TCPMessage
}

func CreateViewMsgCmd(message *TCPMessage) tea.Cmd {
	return func() tea.Msg {
		return ViewMessageMsg{message}
	}
}

func (*MessageViewModel) Init() tea.Cmd {
	return nil
}
func (m *MessageViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ViewMessageMsg:
		m.viewedMessage = msg.message
	}

	return m, nil
}
func (m *MessageViewModel) View() string {
	if m.viewedMessage == nil {
		return "No message to view"
	}

	hexdump := hex.Dump(m.viewedMessage.Content())

	if m.proxy.vieweingMessage {
		hexdump = styles.Selected.Render(hexdump)
	}

	return hexdump
}

func (Console) Init() tea.Cmd                         { return nil }
func (c Console) Update(tea.Msg) (tea.Model, tea.Cmd) { return c, nil }
func (c Console) View() string                        { return c.name + "\n\n" + c.String() }

func MakeModel(proxy *Proxy, debugConsole Console, messageView *MessageViewModel) Model {
	m := Model{
		tui: &boxer.Boxer{},
	}

	m.tui.LayoutTree = boxer.Node{
		VerticalStacked: true,
		Children: []boxer.Node{
			Must(m.tui.CreateLeaf("main", proxy)),
			Must(m.tui.CreateLeaf("messageView", messageView)),
			Must(m.tui.CreateLeaf("debug", debugConsole)),
		},
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return m.tui.ModelMap["main"].Init()
}

func (m Model) UpdateNode(msg tea.Msg, address string) tea.Cmd {
	nodeModel, cmd := m.tui.ModelMap[address].Update(msg)
	m.tui.ModelMap[address] = nodeModel
	return cmd
}

func (m Model) UpdateMultipleNodes(msg tea.Msg, addresses ...string) tea.Cmd {
	var cmds []tea.Cmd
	for _, address := range addresses {
		cmds = append(cmds, m.UpdateNode(msg, address))
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, m.UpdateNode(msg, "main")
	case ViewMessageMsg:
		return m, m.UpdateNode(msg, "messageView")
	case tea.WindowSizeMsg:
		m.tui.UpdateSize(msg)
	case TickMsg, editBufferInEditorMsg:
		return m, m.UpdateNode(msg, "main")
	}
	return m, nil
}

func (m Model) View() string {
	return m.tui.View()
}

func main() {
	proxy := NewProxy(getArgs())
	debugConsole := Console{
		Builder: new(strings.Builder),
		name:    "Debug Console",
	}
	log.SetOutput(debugConsole)

	program := tea.NewProgram(MakeModel(proxy, debugConsole, &MessageViewModel{nil, proxy}), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Printf("There's been an error: %v", err)
		os.Exit(1)
	}
}
