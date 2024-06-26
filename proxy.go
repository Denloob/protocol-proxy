package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Denloob/protocol-proxy/styles"
	"github.com/Denloob/protocol-proxy/tcpmessage"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

type TickMsg time.Time

func Tick() tea.Msg {
	return TickMsg(time.Now())
}

type Proxy struct {
	args                 Args
	messages             []*tcpmessage.TCPMessage
	messagesMutex         sync.RWMutex
	selectedMessageIndex int
	help                 help.Model
	windowSize           tea.WindowSizeMsg
	autoTransmit         bool
}

func NewProxy(args Args) *Proxy {
	return &Proxy{
		args:                 args,
		messages:             nil,
		selectedMessageIndex: -1,
		help:                 help.New(),
		autoTransmit:         false,
	}
}

func (p *Proxy) AutoTransmit() bool {
	return p.autoTransmit
}

func (p *Proxy) SelectedMessage() (*tcpmessage.TCPMessage, error) {
	p.messagesMutex.RLock()
	defer p.messagesMutex.RUnlock()

	if p.selectedMessageIndex == -1 {
		return nil, fmt.Errorf("no message selected")
	}

	return p.messages[p.selectedMessageIndex], nil
}

func (p *Proxy) AddMessage(message *tcpmessage.TCPMessage) {
	p.messagesMutex.Lock()
	defer p.messagesMutex.Unlock()

	p.messages = append(p.messages, message)
}

func (proxy *Proxy) CreateTransmittionHandler(transmittionDirection tcpmessage.TransmittionDirection) func(buffer []byte) []byte {
	return func(buffer []byte) []byte {
		message := tcpmessage.New(transmittionDirection, buffer)

		proxy.AddMessage(message)

		if proxy.AutoTransmit() {
			message.MarkAsTransmited()
		} else {
			transmit := message.WaitForTransmittion()
			if !transmit {
				return nil
			}
		}

		return message.Content()
	}
}

func (p *Proxy) Init() tea.Cmd {
	return func() tea.Msg {
		go p.Run()
		return Tick()
	}
}

type ShowFullHelpMsg struct{}

func ShowFullHelpCmd() tea.Msg {
	return ShowFullHelpMsg{}
}

type AutoTransmitMsg bool

func CreateAutoTransmitCmd(autoTransmit bool) tea.Cmd {
	return func() tea.Msg {
		return AutoTransmitMsg(autoTransmit)
	}
}

func (p *Proxy) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds tea.BatchMsg

	switch msg := msg.(type) {
	case tea.KeyMsg:
		newProxy, cmd := keyMap.Handle(p, msg)
		p = newProxy.(*Proxy)
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		p.windowSize = msg
	case AutoTransmitMsg:
		p.autoTransmit = bool(msg)
	case TickMsg:
		cmds = append(cmds, Tick, p.tick())
	case ShowFullHelpMsg:
		p.help.ShowAll = !p.help.ShowAll
	case editBufferInEditorMsg:
		if msg.err != nil {
			log.Printf("error during message editing: %v\n", msg.err)
		} else {
			err := Must(p.SelectedMessage()).SetContent(msg.newBuffer)
			if err != nil {
				log.Println(err)
			}
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *Proxy) View() string {
	p.messagesMutex.RLock()
	defer p.messagesMutex.RUnlock()

	var res string
	availableLines := p.windowSize.Height - CountLines(p.help.View(keyMap)) - 1

	begin := p.selectedMessageIndex - availableLines/2
	begin = max(begin, 0)
	end := begin + availableLines
	end = min(end, len(p.messages))

	for i, message := range p.messages[begin:end] {
		i += begin // Offset i to be the index in p.messages instead the index in the [begin:end] slice

		line := fmt.Sprintf("%d. %v", i+1, message)

		style := styles.Unstyled

		if i == p.selectedMessageIndex {
			style = styles.Selected
		}

		line = style.Render(line)
		res += line + "\n"
	}

	return PutOnTheBottomOfView(res, p.help.View(keyMap), p.windowSize.Height)
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

			go forward(inConn, outConn, proxy.CreateTransmittionHandler(tcpmessage.TRANSMITTION_DIRECTION_TO_SERVER))
			go forward(outConn, inConn, proxy.CreateTransmittionHandler(tcpmessage.TRANSMITTION_DIRECTION_TO_CLIENT))
		}(conn)
	}
}

// tick "ticks" the state of the Proxy, updating everything that should be
// updated every tick
func (p *Proxy) tick() tea.Cmd {
	p.messagesMutex.RLock()
	defer p.messagesMutex.RUnlock()

	if len(p.messages) > 0 && p.selectedMessageIndex == -1 {
		p.selectedMessageIndex = 0

		return CreateViewMsgCmd(p.messages[p.selectedMessageIndex])
	}

	return nil
}
