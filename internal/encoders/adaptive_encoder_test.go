package encoders

import (
	"bytes"
	"os"
	"testing"
)

func TestAdaptiveDictBasic(t *testing.T) {
	t.Log("---- ADAPTIVE DICT BASIC TEST ----")
	ad := NewAdaptiveDict(100, 20)
	vType, payload := ad.EncodeValue([]byte("long-value"))
	if vType != 0 {
		t.Fatal("expected type 0 for first encounter")
	}
	vType, _ = ad.EncodeValue([]byte("short"))
	if vType != 0 {
		t.Fatal("expected type 0 for short value")
	}
	vType, payload = ad.EncodeValue([]byte("long-value"))
	if vType != 1 {
		t.Fatal("expected type 1 for promoted value")
	}
	decoded, err := ad.DecodeValue(vType, payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != "long-value" {
		t.Fatalf("expected long-value, got %s", string(decoded))
	}
}

func TestAdaptiveDictWindowing(t *testing.T) {
	t.Log("---- ADAPTIVE DICT WINDOWING TEST ----")
	ad := NewAdaptiveDict(5, 100)
	val := []byte("temp-candidate")
	ad.EncodeValue(val)
	for i := 1; i <= 6; i++ {
		ad.EncodeValue([]byte{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9})
	}
	ad.EncodeValue(val)
	h := hashValue(val)
	found := false
	for _, c := range ad.candidates[h] {
		if bytes.Equal(c.value, val) {
			found = true
			if c.freq != 1 {
				t.Fatalf("Expected freq 1 (reset), got %d. Window cleanup failed!", c.freq)
			}
		}
	}
	if !found {
		t.Fatal("Candidate should be present but reset")
	}
}

func TestAdaptiveDictPersistence(t *testing.T) {
	t.Log("---- ADAPTIVE DICT PERSISTENCE TEST ----")
	ad := NewAdaptiveDict(100, 15)
	val := []byte("persistent-val")
	ad.EncodeValue(val)
	ad.EncodeValue(val)

	tmpFile := "test_adaptive.dict"
	defer os.Remove(tmpFile)
	data, err := ad.WriteDictionary()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}
	if err := os.WriteFile(tmpFile, data, 0666); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	newAd := NewAdaptiveDict(100, 15)
	fileData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if err := newAd.ReadDictionary(fileData); err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}
	vType, payload := newAd.EncodeValue(val)
	if vType != 1 {
		t.Fatal("expected value to be recognized as ID after reload")
	}
	decoded, _ := newAd.DecodeValue(vType, payload)
	if !bytes.Equal(decoded, val) {
		t.Fatal("data corrupted after reload")
	}
}

func TestAdaptiveDictErrors(t *testing.T) {
	t.Log("---- ADAPTIVE DICT ERROR HANDLING TEST ----")
	ad := NewAdaptiveDict(100, 20)
	_, err := ad.DecodeValue(1, []byte{99})
	if err == nil {
		t.Fatal("expected error for non-existing ID")
	}
	_, err = ad.DecodeValue(5, []byte{1})
	if err == nil {
		t.Fatal("expected error for invalid value type")
	}
	_, err = ad.DecodeValue(1, []byte{})
	if err == nil {
		t.Fatal("expected error for empty payload with type 1")
	}
}
