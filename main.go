package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Denloob/protocol-proxy/symbols"
	"github.com/Denloob/protocol-proxy/tcpmessage"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	boxer "github.com/treilik/bubbleboxer"
)

const DEFAULT_EXTRACT_STRINGS_MIN_LENGTH = 4

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
			if len(newBuffer) == 0 { // Message dropped, skip
				return
			}

			_, err := dest.Write(newBuffer)

			if err != nil {
				log.Printf("Write failed: %v", err)
			}
		}()
	}
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

type KeyMap interface {
	Handle(model tea.Model, keyMsg tea.KeyMsg) (tea.Model, tea.Cmd)

	ShortHelp() []key.Binding
	FullHelp() [][]key.Binding
}

type MainKeyMap struct {
	Quit,
	Help,
	Up,
	Down,
	MessageUp,
	MessageDown,
	DisplayHex,
	DisplayHexdump,
	DisplayStrings,
	Drop,
	Transmit,
	Edit key.Binding
}

func NewMainKeymap() *MainKeyMap {
	return &MainKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "show extended help"),
		),
		DisplayHex: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "show message hex"),
		),
		DisplayHexdump: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "show message hexdump"),
		),
		DisplayStrings: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "show message strings"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp(symbols.CurrentMap[symbols.ScArrowUp]+"/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp(symbols.CurrentMap[symbols.ScArrowDown]+"/j", "move down"),
		),
		MessageUp: key.NewBinding(
			key.WithKeys("K", "shift+up"),
			key.WithHelp(symbols.CurrentMap[symbols.ScShift]+"+"+symbols.CurrentMap[symbols.ScArrowUp]+"/K", "move down"),
		),
		MessageDown: key.NewBinding(
			key.WithKeys("J", "shift+down"),
			key.WithHelp(symbols.CurrentMap[symbols.ScShift]+"+"+symbols.CurrentMap[symbols.ScArrowDown]+"/J", "move down"),
		),
		Drop: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "drop"),
		),
		Transmit: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "transmit"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
	}
}

func (k *MainKeyMap) Handle(model tea.Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	proxy := model.(*Proxy)
	selectedMessageChanged := false

	switch {
	case key.Matches(msg, k.Quit):
		return proxy, tea.Quit
	case key.Matches(msg, k.Help):
		return proxy, ShowFullHelpCmd
	case key.Matches(msg, k.Up) && proxy.selectedMessageIndex > 0:
		proxy.selectedMessageIndex--
		selectedMessageChanged = true
	case key.Matches(msg, k.MessageUp):
		return proxy, CreateScrollMessageViewCmd(ScrollMessageViewUp)
	case key.Matches(msg, k.MessageDown):
		return proxy, CreateScrollMessageViewCmd(ScrollMessageViewDown)
	case key.Matches(msg, k.DisplayHex):
		return proxy, CreateChangeMessageDisplayMethodCmd(MESSAGE_DISPLAY_METHOD_HEX)
	case key.Matches(msg, k.DisplayHexdump):
		return proxy, CreateChangeMessageDisplayMethodCmd(MESSAGE_DISPLAY_METHOD_HEXDUMP)
	case key.Matches(msg, k.DisplayStrings):
		return proxy, CreateChangeMessageDisplayMethodCmd(MESSAGE_DISPLAY_METHOD_STRINGS)
	case key.Matches(msg, k.Down) && proxy.selectedMessageIndex < len(proxy.messages)-1:
		proxy.selectedMessageIndex++
		selectedMessageChanged = true
	case key.Matches(msg, k.Drop), key.Matches(msg, k.Transmit), key.Matches(msg, k.Edit):
		message, err := proxy.SelectedMessage()
		if err != nil {
			log.Println(err)
			return proxy, nil
		}

		switch {
		case key.Matches(msg, k.Drop):
			err := message.Drop()
			if err != nil {
				log.Printf("Drop error: %s\n", err)
				return proxy, nil
			}
		case key.Matches(msg, k.Transmit):
			err := message.Transmit()
			if err != nil {
				log.Printf("Transmittion error: %s\n", err)
				return proxy, nil
			}
		case key.Matches(msg, k.Edit):
			messageContent := message.Content()
			cmd, err := editBufferInEditor(messageContent)
			if err != nil {
				log.Printf("Edit in editor error: %s\n", err)
				return proxy, nil
			}

			return proxy, cmd
		}
	}

	if selectedMessageChanged {
		currentMessage := Must(proxy.SelectedMessage())
		return proxy, CreateViewMsgCmd(currentMessage)
	}

	return proxy, nil
}

func (k MainKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Help}
}

func (k MainKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Transmit, k.Edit, k.Drop},
		{k.MessageUp, k.MessageDown},
		{k.DisplayHex, k.DisplayHexdump, k.DisplayStrings},
		{k.Quit, k.Help},
	}
}

var keyMap KeyMap

type MessageDisplayMethod int

const (
	MESSAGE_DISPLAY_METHOD_HEXDUMP MessageDisplayMethod = iota
	MESSAGE_DISPLAY_METHOD_STRINGS
	MESSAGE_DISPLAY_METHOD_HEX
)

func CreateChangeMessageDisplayMethodCmd(method MessageDisplayMethod) tea.Cmd {
	return func() tea.Msg {
		return method
	}
}

type MessageViewModel struct {
	viewedMessage *tcpmessage.TCPMessage

	displayMethod MessageDisplayMethod
	windowSize    tea.WindowSizeMsg
	scroll        int
}

type ViewMessageMsg struct {
	message *tcpmessage.TCPMessage
}

type ScrollMessageViewMsg int

const (
	ScrollMessageViewUp   = -1
	ScrollMessageViewDown = 1
)

func CreateScrollMessageViewCmd(scroll int) tea.Cmd {
	return func() tea.Msg {
		return ScrollMessageViewMsg(scroll)
	}
}

func CreateViewMsgCmd(message *tcpmessage.TCPMessage) tea.Cmd {
	return func() tea.Msg {
		return ViewMessageMsg{message}
	}
}

func (m *MessageViewModel) maxScroll() int {
	return max(CountLines(m.renderWrapped())-m.windowSize.Height-1, 0)
}

func (*MessageViewModel) Init() tea.Cmd {
	return nil
}
func (m *MessageViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowSize = msg
	case ViewMessageMsg:
		m.viewedMessage = msg.message
		m.scroll = 0
	case MessageDisplayMethod:
		m.displayMethod = msg
		m.scroll = 0
	case ScrollMessageViewMsg:
		m.scroll += int(msg)
		m.scroll = Clamp(m.scroll, 0, m.maxScroll())
	}

	return m, nil
}
func (m *MessageViewModel) View() string {
	lines := strings.Split(m.renderWrapped(), "\n")

	begin := m.scroll
	end := min(begin+m.windowSize.Height, len(lines))
	return strings.Join(lines[begin:end], "\n")
}

func (m *MessageViewModel) renderWrapped() string {
	lines := strings.Split(m.render(), "\n")

	// Wrap is not supported for hexdump display
	if m.displayMethod != MESSAGE_DISPLAY_METHOD_HEXDUMP {
		var wrappedLines [][]string
		for _, line := range lines {
			wrappedLines = append(wrappedLines, WrapLine(line, m.windowSize.Width))
		}
		lines = Flatten(wrappedLines)
	}

	return strings.Join(lines, "\n")
}

func (m *MessageViewModel) render() string {
	if m.viewedMessage == nil {
		return "No message to view"
	}

	messageContent := m.viewedMessage.Content()

	switch m.displayMethod {
	case MESSAGE_DISPLAY_METHOD_HEXDUMP:
		return hex.Dump(messageContent)
	case MESSAGE_DISPLAY_METHOD_STRINGS:
		extractedStrings := ExtractStrings([]byte(messageContent), DEFAULT_EXTRACT_STRINGS_MIN_LENGTH)
		return strings.Join(extractedStrings, "\n")
	case MESSAGE_DISPLAY_METHOD_HEX:
		return fmt.Sprintf("%x", messageContent)
	default:
		panic("invalid message display method")
	}
}

type Model struct {
	tui *boxer.Boxer
}

func MakeModel(proxy *Proxy, debugConsole *Console, messageView *MessageViewModel) Model {
	m := Model{
		tui: &boxer.Boxer{},
	}

	m.tui.LayoutTree = boxer.Node{
		SizeFunc: func(node boxer.Node, height int) []int {
			return []int{
				height / 2,
				height / 4,
				height / 4,
			}
		},
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
	case ViewMessageMsg, MessageDisplayMethod, ScrollMessageViewMsg:
		return m, m.UpdateNode(msg, "messageView")
	case tea.WindowSizeMsg:
		m.tui.UpdateSize(msg)
		m.UpdateNode(tea.WindowSizeMsg{Height: msg.Height/2 - 1, Width: msg.Width}, "main")
		m.UpdateNode(tea.WindowSizeMsg{Height: msg.Height/4 - 1, Width: msg.Width}, "messageView")
		m.UpdateNode(tea.WindowSizeMsg{Height: msg.Height/4 - 1, Width: msg.Width}, "debug")
	case TickMsg, editBufferInEditorMsg, ShowFullHelpMsg:
		return m, m.UpdateNode(msg, "main")
	}
	return m, nil
}

func (m Model) View() string {
	return m.tui.View()
}

func main() {
	symbols.CurrentMap = symbols.NerdFontMap

	keyMap = NewMainKeymap()

	proxy := NewProxy(getArgs())
	debugConsole := NewConsole("Debug Console")
	log.SetOutput(debugConsole)

	program := tea.NewProgram(MakeModel(proxy, debugConsole, &MessageViewModel{}), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Printf("There's been an error: %v", err)
		os.Exit(1)
	}
}
