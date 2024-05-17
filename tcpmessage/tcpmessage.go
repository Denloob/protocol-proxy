package tcpmessage

import (
	"fmt"
	"time"

	"github.com/Denloob/protocol-proxy/symbols"
)

type Status int

const (
	STATUS_PENDING Status = iota
	STATUS_TRANSMITED
	STATUS_DROPPED
)

func (status Status) String() string {
	switch status {
	case STATUS_PENDING:
		return symbols.CurrentMap[symbols.ScClock]
	case STATUS_TRANSMITED:
		return symbols.CurrentMap[symbols.ScSentMail]
	case STATUS_DROPPED:
		return symbols.CurrentMap[symbols.ScTrashCan]
	default:
		panic("Invalid status")
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
		return symbols.CurrentMap[symbols.ScArrowLeft]
	case TRANSMITTION_DIRECTION_TO_CLIENT:
		return symbols.CurrentMap[symbols.ScArrowRight]
	default:
		panic("Invalid direction")
	}
}

type TCPMessage struct {
	content   []byte
	edited    bool
	status    Status
	time      time.Time
	direction TransmittionDirection

	transmitChan chan bool
}

func New(transmittionDirection TransmittionDirection, content []byte) *TCPMessage {
	return &TCPMessage{
		content:   content,
		edited:    false,
		status:    STATUS_PENDING,
		time:      time.Now(),
		direction: transmittionDirection,

		transmitChan: make(chan bool),
	}
}

// WaitForTransmittion waits for a signal to transmit. If the signal is true,
// the message content shall be transmitted. Otherwise, it shall be dropped.
func (message *TCPMessage) WaitForTransmittion() (transmit bool) {
	return <-message.transmitChan
}

// MarkAsTransmited marks the message as transmited without notifying anobody
// waiting for transmittion. Use only when nobody is waiting for transmittion.
// no calls to Transmit/MarkAsTransmited will be possible after this call.
func (message *TCPMessage) MarkAsTransmited() error {
	switch message.status {
	case STATUS_PENDING:
		message.status = STATUS_TRANSMITED

	case STATUS_TRANSMITED:
		return fmt.Errorf("The message was already transmitted. Can't retransmit.")
	case STATUS_DROPPED:
		return fmt.Errorf("The message was dropped. Can't transmit.")
	default:
		panic("Invalid status")
	}

	return nil
}

// Transmit marks the packet as transmited and notifies everybody waiting with
// WaitForTransmittion about the transmittion.
func (message *TCPMessage) Transmit() error {
	err := message.MarkAsTransmited()
	if err != nil {
		return err
	}

	message.transmitChan <- true

	return nil
}

func (message *TCPMessage) Drop() error {
	switch message.status {
	case STATUS_PENDING:
		message.status = STATUS_DROPPED

	case STATUS_TRANSMITED:
		return fmt.Errorf("The message was already transmitted. Can't drop.")
	case STATUS_DROPPED:
		return fmt.Errorf("The message was dropped. Can't drop again.")
	default:
		panic("Invalid status")
	}

	message.transmitChan <- false

	return nil
}

func (message *TCPMessage) SetContent(newContent []byte) error {
	if message.status != STATUS_PENDING {
		return fmt.Errorf("The message can no longer be edited.")
	}

	message.content = newContent
	message.edited = true

	return nil
}

func (message *TCPMessage) Content() []byte {
	return message.content
}

func (message *TCPMessage) String() string {
	messageState := message.status.String()
	if message.edited {
		messageState += " " + symbols.CurrentMap[symbols.ScPen]
	}

	return fmt.Sprintf("[%v] %v %v (%v bytes)", message.time.Format(time.TimeOnly), messageState, message.direction, len(message.content))
}
