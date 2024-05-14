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

	"github.com/Denloob/protocol-proxy/styles"
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
		key.WithHelp(symbols.CurrentMap[symbols.ScArrowUp]+"/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp(symbols.CurrentMap[symbols.ScArrowDown]+"/j", "move down"),
	),
	View: key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp(symbols.CurrentMap[symbols.ScSpace]+"/"+symbols.CurrentMap[symbols.ScEnter], "transmit"),
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
		currentMessage := Must(proxy.SelectedMessage())
		return proxy, CreateViewMsgCmd(currentMessage)
	}

	return proxy, nil
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
		message, err := proxy.SelectedMessage()
		if err != nil {
			log.Printf("Can't edit message: %v", err)
			return proxy, nil
		}

		messageContent := message.Content()
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
	viewedMessage *tcpmessage.TCPMessage

	proxy *Proxy
}

type ViewMessageMsg struct {
	message *tcpmessage.TCPMessage
}

func CreateViewMsgCmd(message *tcpmessage.TCPMessage) tea.Cmd {
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

	if m.proxy.VieweingMessage() {
		hexdump = styles.Selected.Render(hexdump)
	}

	return hexdump
}

type Model struct {
	tui *boxer.Boxer
}

func MakeModel(proxy *Proxy, debugConsole *Console, messageView *MessageViewModel) Model {
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
	symbols.CurrentMap = symbols.NerdFontMap

	proxy := NewProxy(getArgs())
	debugConsole := NewConsole("Debug Console")
	log.SetOutput(debugConsole)

	program := tea.NewProgram(MakeModel(proxy, debugConsole, &MessageViewModel{nil, proxy}), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Printf("There's been an error: %v", err)
		os.Exit(1)
	}
}
