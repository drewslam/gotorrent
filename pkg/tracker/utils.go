/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"fmt"
	"strings"
)

const DefaultPort = 6881
const IdPrefix = "DSGT01"

func EscapeBytes(data []byte) string {
	var res strings.Builder
	for _, d := range data {
		res.WriteString(fmt.Sprintf("%%%02X", d))
	}
	return res.String()
}


