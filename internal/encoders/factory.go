package encoders

import "errors"

const (
	NoCompression        byte = 0
	PrefixCompression    byte = 1
	DictDeltaCompression byte = 2
)

func NewEncoder(t byte, restartInterval int) (Encoder, error) {
	switch t {
	case NoCompression:
		return nil, nil
	case PrefixCompression:
		return NewDeltaEncoderBytes(restartInterval), nil
	case DictDeltaCompression:
		return NewDictDeltaEncoder(restartInterval), nil
	default:
		return nil, errors.New("unknown encoder type")

	}
}
