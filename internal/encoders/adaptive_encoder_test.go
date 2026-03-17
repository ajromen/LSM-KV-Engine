package encoders

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestAdaptiveDictBasic(t *testing.T) {
	t.Log("---- ADAPTIVE DICT BASIC TEST ----")
	ad := NewAdaptiveDictEncoderFrequency(2, 20)

	_, promoted := ad.AddToDict([]byte("long-value"))
	payload := ad.Encode([]byte("long-value"))
	if promoted && payload[0] != 1 {
		t.Fatalf("expected dictionary encoding after promotion")
	}

	_, promoted = ad.AddToDict([]byte("short"))
	payload = ad.Encode([]byte("short"))
	if promoted && payload[0] != 1 {
		t.Fatalf("expected raw encoding for first encounter")
	}

	_, promoted = ad.AddToDict([]byte("long-value"))
	payload = ad.Encode([]byte("long-value"))
	if payload[0] != 1 {
		t.Fatalf("expected dictionary encoding after second promotion, got %d", payload[0])
	}

	decoded, err := ad.Decode(payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != "long-value" {
		t.Fatalf("expected long-value, got %s", string(decoded))
	}
}

func TestAdaptiveDictWindowing(t *testing.T) {
	t.Log("---- ADAPTIVE DICT WINDOWING TEST ----")
	ad := NewAdaptiveDictEncoderFrequency(5, 5)
	val := []byte("temp-candidate")

	ad.AddToDict(val)
	ad.Encode(val)

	for i := 1; i <= 6; i++ {
		ad.AddToDict([]byte{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9})
	}

	ad.AddToDict(val)
	payload := ad.Encode(val)
	if payload[0] != 0 {
		t.Fatal("expected raw encoding after candidate expired from window")
	}
}

func TestAdaptiveDictPersistence(t *testing.T) {
	t.Log("---- ADAPTIVE DICT PERSISTENCE TEST ----")
	ad := NewAdaptiveDictEncoderFrequency(2, 15)
	val := []byte("persistent-val")

	ad.AddToDict(val)
	ad.AddToDict(val)
	payload := ad.Encode(val)
	if payload[0] != 1 {
		t.Fatal("expected dictionary encoding after promotion")
	}

	tmpFile := t.TempDir() + "/adaptive.dict"
	f, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	if err := ad.WriteToFile(f, 0); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	newAd := NewAdaptiveDictEncoderFrequency(2, 15)
	fRead, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer fRead.Close()

	info, _ := fRead.Stat()
	if err := newAd.ReadFromFile(fRead, 0, int(info.Size())); err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	payload = newAd.Encode(val)
	if payload[0] != 1 {
		t.Fatal("expected dictionary encoding after reload")
	}

	decoded, err := newAd.Decode(payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bytes.Equal(decoded, val) {
		t.Fatal("data corrupted after reload")
	}
}

func TestMyProgram(t *testing.T) {
	ad := NewAdaptiveDictEncoderFrequency(3, 3)
	val := []byte("persistent-val")
	ad.AddToDict(val)
	ad.AddToDict(val)
	payload := ad.Encode(val)
	fmt.Println(payload)
	valnon := []byte("non-persistent-val")
	ad.AddToDict(valnon)
	payload = ad.Encode(valnon)
	fmt.Println(payload)
	valtest := []byte("testing")
	ad.AddToDict(valtest)
	ad.AddToDict(valnon)
	ad.AddToDict(valnon)
	ad.AddToDict(val)
	ad.AddToDict(valtest)
	ad.AddToDict(valtest)
	payload = ad.Encode(valtest)
	fmt.Println(payload)
}
