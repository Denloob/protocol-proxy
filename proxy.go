package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Denloob/protocol-proxy/styles"
	"github.com/Denloob/protocol-proxy/tcpmessage"

	tea "github.com/charmbracelet/bubbletea"
)

type TickMsg time.Time

func Tick() tea.Msg {
	return TickMsg(time.Now())
}

type Proxy struct {
	args                 Args
	messages             []*tcpmessage.TCPMessage
	vieweingMessage      bool
	selectedMessageIndex int
}

func NewProxy(args Args) *Proxy {
	return &Proxy{
		args:                 args,
		messages:             nil,
		vieweingMessage:      false,
		selectedMessageIndex: -1,
	}
}

func (p *Proxy) VieweingMessage() bool {
	return p.vieweingMessage
}

func (p *Proxy) SelectedMessage() (*tcpmessage.TCPMessage, error) {
	if p.selectedMessageIndex == -1 {
		return nil, fmt.Errorf("no message selected")
	}

	return p.messages[p.selectedMessageIndex], nil
}

func (p *Proxy) AddMessage(message *tcpmessage.TCPMessage) {
	p.messages = append(p.messages, message)
}

func (proxy *Proxy) CreateTransmittionHandler(transmittionDirection tcpmessage.TransmittionDirection) func(buffer []byte) []byte {
	return func(buffer []byte) []byte {
		message := tcpmessage.New(transmittionDirection, buffer)

		proxy.AddMessage(message)

		message.WaitForTransmittion()

		return message.Content()
	}
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
		cmds = append(cmds, Tick, p.tick())
	case editBufferInEditorMsg:
		if msg.err != nil {
			log.Printf("error during message editing: %v\n", msg.err)
		} else {
			Must(p.SelectedMessage()).SetContent(msg.newBuffer)
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

			go forward(inConn, outConn, proxy.CreateTransmittionHandler(tcpmessage.TRANSMITTION_DIRECTION_TO_SERVER))
			go forward(outConn, inConn, proxy.CreateTransmittionHandler(tcpmessage.TRANSMITTION_DIRECTION_TO_CLIENT))
		}(conn)
	}
}

// tick "ticks" the state of the Proxy, updating everything that should be
// updated every tick
func (p *Proxy) tick() tea.Cmd {
	if len(p.messages) > 0 && p.selectedMessageIndex == -1 {
		p.selectedMessageIndex = 0

		return CreateViewMsgCmd(p.messages[p.selectedMessageIndex])
	}

	return nil
}