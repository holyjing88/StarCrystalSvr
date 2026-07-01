package starcrystaljson

import (
	"bytes"
	"os"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ReadFileUTF8 reads the entire file as UTF-8 and strips an optional UTF-8 BOM prefix.
// encoding/json rejects a leading BOM; editors often save "UTF-8 with signature" that way.
func ReadFileUTF8(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(raw, utf8BOM), nil
}
