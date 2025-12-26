/*
 * title: gotorrent-tracker utils
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"fmt"
	"math/rand"
)

const DefaultPort uint16 = 6881
const IdPrefix string = "DSGT01"

type Event int
const (
	EventStarted Event = iota
	EventStopped
	EventCompleted
)

func (e Event) String() (string, error) {
	switch e {
	case EventStarted:
		return "started", nil
	case EventStopped:
		return "stopped", nil
	case EventCompleted:
		return "completed", nil
	default:
		return "", fmt.Errorf("invalid event type")
	}
}

func (e Event) Uint32() (uint32, error) {
	switch e {
	case EventStarted:
		return 2, nil
	case EventStopped:
		return 3, nil
	case EventCompleted:
		return 1, nil
	default:
		return 0, fmt.Errorf("invalid event type")
	}
}

func NewUint32() uint32 {
	return rand.Uint32()
}
