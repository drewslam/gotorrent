/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"fmt"
	"math/rand"
	"strings"
)

const DefaultPort uint16 = 6881
const IdPrefix string = "DSGT01"

func EscapeBytes(data []byte) string {
	var res strings.Builder
	for _, d := range data {
		res.WriteString(fmt.Sprintf("%%%02X", d))
	}
	return res.String()
}

func NewTransactionID() uint32 {
	return rand.Uint32()
}
