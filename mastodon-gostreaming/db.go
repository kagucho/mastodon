package main

import "strconv"

type pqInt64Buffer []byte

func newPQInt64Buffer(number int) pqInt64Buffer {
	return pqInt64Buffer(append(make([]byte, 0, number*8), '{'))
}

func (buffer *pqInt64Buffer) finalize() []byte {
	bytes := []byte(*buffer)
	if len(bytes) > 1 {
		bytes[len(bytes)-1] = '}'
	} else {
		bytes = append(bytes, '}')
	}

	return bytes
}

func (buffer *pqInt64Buffer) write(id int64) {
	*buffer = pqInt64Buffer(append(strconv.AppendInt([]byte(*buffer), id, 10), ','))
}
