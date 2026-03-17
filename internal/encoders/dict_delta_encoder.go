package encoders

type DictDeltaEncoder struct {
	dictEncoder  *DictEncoder
	deltaEncoder *DeltaEncoderInt
}

func NewDictDeltaEncoder(restartInterval int) *DictDeltaEncoder {
	return &DictDeltaEncoder{
		dictEncoder:  NewDictEncoder(nil),
		deltaEncoder: NewDeltaEncoderInt(restartInterval),
	}
}

func (e *DictDeltaEncoder) Reset() {
	e.deltaEncoder.Reset()
}

// Encode string key -> dictionary ID -> delta encode
func (e *DictDeltaEncoder) Encode(key []byte, blockOffset uint32, buf []byte) []byte {
	id, _ := e.dictEncoder.AddToDict(key)
	return e.deltaEncoder.Encode(int(id), blockOffset, buf)
}

// Decode delta -> dictionary ID -> string key
func (e *DictDeltaEncoder) Decode(buf []byte, pos *int) ([]byte, error) {
	id, err := e.deltaEncoder.Decode(buf, pos)
	if err != nil {
		return nil, err
	}
	// DictEncoder.Key expects uint64
	return []byte(e.dictEncoder.Key(uint64(id))), nil
}

// SerializeDictionary returns dictionary as bytes
func (e *DictDeltaEncoder) SerializeDictionary() []byte {
	return e.dictEncoder.SaveDict()
}

// LoadDictionary loads dictionary from bytes
func (e *DictDeltaEncoder) LoadDictionary(buf []byte) error {
	e.dictEncoder = NewDictEncoder(buf)
	return nil
}

// WriteRestartArray proxies to deltaEncoder
func (e *DictDeltaEncoder) WriteRestartArray(buf []byte) []byte {
	return e.deltaEncoder.WriteRestartArray(buf)
}

// DictionarySize returns number of keys in dictionary
func (e *DictDeltaEncoder) DictionarySize() int {
	return e.dictEncoder.Size()
}

// RestartArray returns deltaEncoder restart positions
func (e *DictDeltaEncoder) RestartArray() []uint32 {
	return e.deltaEncoder.RestartArray()
}
