package encoders

const (
	NoCompression        byte = 0
	PrefixCompression    byte = 1
	DictDeltaCompression byte = 2
)

func NewEncoder(t byte, restartInterval int) Encoder {
	switch t {
	case NoCompression:
		return nil
	case PrefixCompression:
		return NewDeltaEncoderBytes(restartInterval)
	case DictDeltaCompression:
		return NewDictDeltaEncoder(restartInterval)
	default:
		return NewDeltaEncoderBytes(restartInterval)
	}
}
