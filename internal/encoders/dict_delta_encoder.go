package encoders

type DictDeltaEncoder struct {
	dictEncoder  *DictionaryEncoder
	deltaEncoder *DeltaEncoderInt
}

func NewDictDeltaEncoder(restartInterval int) *DictDeltaEncoder {
	return &DictDeltaEncoder{
		dictEncoder:  NewDictionaryEncoder(),
		deltaEncoder: NewDeltaEncoderInt(restartInterval),
	}
}

func (e *DictDeltaEncoder) Reset() {
	e.deltaEncoder.Reset()
}

// Encode string key -> dictionary ID -> delta encode
func (e *DictDeltaEncoder) Encode(key []byte, blockOffset uint32, buf []byte) []byte {
	id, _ := e.dictEncoder.AddToDict(key)
	return e.deltaEncoder.Encode(id, blockOffset, buf)
}

// Decode delta -> dictionary ID -> string key
func (e *DictDeltaEncoder) Decode(buf []byte, pos *int) ([]byte, error) {
	id, err := e.deltaEncoder.Decode(buf, pos)
	if err != nil {
		return nil, err
	}
	return e.dictEncoder.GetKey(id)
}

func (e *DictDeltaEncoder) SerializeDictionary() []byte {
	return e.dictEncoder.Serialize()
}

func (e *DictDeltaEncoder) LoadDictionary(buf []byte) error {
	d, err := Deserialize(buf)
	if err != nil {
		return err
	}
	e.dictEncoder = d
	return nil
}

func (e *DictDeltaEncoder) WriteRestartArray(buf []byte) []byte {
	return e.deltaEncoder.WriteRestartArray(buf)
}

func (e *DictDeltaEncoder) DictionarySize() int {
	return e.dictEncoder.Size()
}

func (e *DictDeltaEncoder) RestartArray() []uint32 {
	return e.deltaEncoder.restartArray
}
