package encoders

type Encoder interface {
	Encode(key []byte, blockOffset uint32, buf []byte) []byte
	Decode(buf []byte, pos *int) ([]byte, error)
	Reset()
	RestartArray() []uint32
	RestartInterval() int
}
