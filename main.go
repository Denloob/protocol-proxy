package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

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

type QuestionDefault int

const (
	QUESTION_WITHOUT_DEFAULT QuestionDefault = iota
	QUESTION_DEFAULT_YES
	QUESTION_DEFAULT_NO
)

func ToUpper(char byte) byte {
	return byte(unicode.ToUpper(rune(char)))
}

func askYesNo(question string, defaultAnswer QuestionDefault) bool {

	switch defaultAnswer {
	case QUESTION_WITHOUT_DEFAULT:
		question += " (y/n) "
	case QUESTION_DEFAULT_YES:
		question += " (Y/n) "
	case QUESTION_DEFAULT_NO:
		question += " (y/N) "
	}

	for {
		fmt.Print(question)

		var input string
		fmt.Scanln(&input)

		if len(input) != 0 {
			switch ToUpper(input[0]) {
			case 'Y':
				return true
			case 'N':
				return false
			}
		}

		if defaultAnswer == QUESTION_WITHOUT_DEFAULT {
			continue
		}

		return defaultAnswer == QUESTION_DEFAULT_YES
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

func inputAction() byte {
	for {
		fmt.Println("Action? [D]rop/view [H]ex/view hexdum[P]/view [S]trings/he[X] overwrite/[A]scii overwrite/open in [E]ditor/[N]othing")
		var input string
		fmt.Scanln(&input)
		if len(input) == 0 {
			continue
		}

		choice := ToUpper(input[0])

		switch choice {
		case 'D', 'H', 'P', 'S', 'O', 'X', 'A', 'E', 'N':
			return choice
		default:
			fmt.Printf("Invalid action: %c\n", choice)
		}
	}
}

func executeAction(action byte, buffer []byte) []byte {
	switch action {
	case 'D':
		fmt.Println("The packet was dropped.")
		return nil
	case 'N':
		return buffer
	case 'H':
		fmt.Printf("%x\n", buffer)
		return buffer
	case 'P':
		fmt.Println(hex.Dump(buffer))
		return buffer
	case 'S':
		extractedStrings := extractStrings(buffer, DEFAULT_EXTRACT_STRINGS_MIN_LENGTH)
		if len(extractedStrings) > 0 {
			fmt.Printf("%d strings found\n---\n", len(extractedStrings))
			fmt.Println(strings.Join(extractedStrings, "\n"))
			fmt.Printf("\n")
		} else {
			fmt.Println("No strings found")
		}
		return buffer
	case 'X':
		var input string
		fmt.Println("Please enter the hex string to overwrite the packet with:")
		fmt.Scanln(&input)
		newBuffer := make([]byte, hex.DecodedLen(len(input)))
		size, err := hex.Decode(newBuffer, []byte(input))
		if err != nil {
			fmt.Println(err)
			return buffer
		}
		return newBuffer[:size]
	case 'A':
		fmt.Println(`NOTE: use \n for new lines, \\n for literal '\n'. Entering a new line will send the packet.`)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			log.Printf("Failed to read a line: %v", scanner.Err())
			return buffer
		}

		input := scanner.Text()
		newlineRegex := regexp.MustCompile(`[^\\]\\n`)
		input = newlineRegex.ReplaceAllString(input, "\n")
		return []byte(input)
	case 'E':
		tempfile, err := os.CreateTemp("", "hexdump")
		if err != nil {
			log.Println(err)
			return buffer
		}
		defer os.Remove(tempfile.Name())

		filename := tempfile.Name()

		tempfile.Write(buffer)
		tempfile.Close()

		editor := os.Getenv("EDITOR")

		cmd := exec.Command(editor, filename)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Run()

		newBuffer, err := os.ReadFile(filename)
		if err != nil {
			log.Println(err)
			return buffer
		}
		return newBuffer
	default:
		panic(fmt.Sprintf("Invalid action: %c", action))
	}
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

func (proxy *ProxyModel) createTransmittionHandler(transmittionDirection TransmittionDirection) func(buffer []byte) []byte {
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

type KeyMap struct {
	Quit,
	Transmit,
	Up,
	Down key.Binding
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
	args     Args
	messages []*TCPMessage
}

type ProxyModel struct {
	*Proxy
	selectedMessageIndex int
}

type TickMsg time.Time

func Tick() tea.Msg {
	return TickMsg(time.Now())
}

func MakeProxyModel(proxy *Proxy) ProxyModel {
	return ProxyModel{Proxy: proxy, selectedMessageIndex: -1}
}

func (p ProxyModel) Init() tea.Cmd {
	return func() tea.Msg {
		go p.Run()
		return Tick()
	}
}
func (proxy ProxyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds tea.BatchMsg
	selectedMessageChanged := false

	if len(proxy.messages) > 0 && proxy.selectedMessageIndex == -1 {
		proxy.selectedMessageIndex = 0
		selectedMessageChanged = true
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyMap.Up) && proxy.selectedMessageIndex > 0:
			proxy.selectedMessageIndex--
			selectedMessageChanged = true
		case key.Matches(msg, keyMap.Down) && proxy.selectedMessageIndex < len(proxy.messages)-1:
			proxy.selectedMessageIndex++
			selectedMessageChanged = true
		case key.Matches(msg, keyMap.Transmit) && len(proxy.messages) > 0:
			err := proxy.messages[proxy.selectedMessageIndex].Transmit()
			if err != nil {
				log.Println(err)
			}
		}
	case TickMsg:
		cmds = append(cmds, Tick)
	}

	if selectedMessageChanged {
		cmds = append(cmds, CreateViewMsgCmd(proxy.messages[proxy.selectedMessageIndex]))
	}

	return proxy, tea.Batch(cmds...)
}
func (p ProxyModel) View() string {
	var res string
	for i, message := range p.messages {
		line := fmt.Sprintf("%d. %v", i+1, message)

		if i == p.selectedMessageIndex {
			line = styles.Selected.Render(line)
		}

		res += line + "\n"
	}

	return res
}
func (proxy ProxyModel) Run() {
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

var keyMap = KeyMap{
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
	Transmit: key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp(symbolMap[symbols.ScSpace]+"/"+symbolMap[symbols.ScEnter], "transmit"),
	),
}

type MessageViewModel struct {
	viewedMessage *TCPMessage
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

	return hex.Dump(m.viewedMessage.Content())
}

var messageView = &MessageViewModel{}

func (Console) Init() tea.Cmd                         { return nil }
func (c Console) Update(tea.Msg) (tea.Model, tea.Cmd) { return c, nil }
func (c Console) View() string                        { return c.name + "\n\n" + c.String() }

func MakeModel(proxy ProxyModel, debugConsole Console) Model {
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
		switch {
		case key.Matches(msg, keyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, keyMap.Up),
			key.Matches(msg, keyMap.Down),
			key.Matches(msg, keyMap.Transmit):

			return m, m.UpdateNode(msg, "main")
		}
	case ViewMessageMsg:
		return m, m.UpdateNode(msg, "messageView")
	case tea.WindowSizeMsg:
		m.tui.UpdateSize(msg)
	case TickMsg:
		return m, m.UpdateNode(msg, "main")
	}
	return m, nil
}

func (m Model) View() string {
	return m.tui.View()
}

func main() {
	proxy := MakeProxyModel(&Proxy{args: getArgs()})
	debugConsole := Console{
		Builder: new(strings.Builder),
		name:    "Debug Console",
	}
	log.SetOutput(debugConsole)

	program := tea.NewProgram(MakeModel(proxy, debugConsole))
	if _, err := program.Run(); err != nil {
		log.Printf("There's been an error: %v", err)
		os.Exit(1)
	}
}
