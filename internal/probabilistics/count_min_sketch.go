package probabilistics

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type CountMinSketch struct {
	k             uint //braco ovo vam je za broj redova, mada malo sam zapisao i zbog sebe
	m             uint //ovo vam je braco za broj kolona
	confidence    float64
	accuracy      float64
	table         [][]uint       //ovde su svi kountovi u matrici
	hashFunctions []HashWithSeed //ovde su vam svi seedovi u fazonu HashWithSeed{Seed: {1,2,3,4}}... i sad napravite listu od ovakvih samo drugaciji Seed:
}

func (c *CountMinSketch) calculateM() {
	c.m = uint(math.Ceil(math.E / c.accuracy))
}

func (c *CountMinSketch) calculateK() {
	delta := 1 - c.confidence
	c.k = uint(math.Ceil(math.Log(1 / delta)))
}

func NewCountMinSketch(config config.CountMinSketchConfig) *CountMinSketch {
	cms := &CountMinSketch{
		accuracy:   config.Accuracy,
		confidence: config.Confidence,
	}
	cms.calculateM()
	cms.calculateK()
	matrix := make([][]uint, cms.k)

	for i := uint(0); i < cms.k; i++ {
		matrix[i] = make([]uint, cms.m)
	}

	cms.table = matrix
	cms.hashFunctions = CreateHashFunctions(config.Seeds)

	return cms
}

func (c *CountMinSketch) Add(key []byte, count uint) {
	for i := uint(0); i < c.k; i++ {
		hash := c.hashFunctions[i].Hash(key)
		col := hash % uint64(c.m)
		c.table[i][col] += count
	}
}

func (c *CountMinSketch) Increment(key []byte) {
	c.Add(key, 1)
}

// OVO VAM JE BRAco glavni deo cms-a, on vam vraca najmanju vrednost za neki key iz sva tri reda
func (c *CountMinSketch) Estimate(key []byte) uint {
	var min uint
	first := true

	for i := uint(0); i < c.k; i++ {
		hash := c.hashFunctions[i].Hash(key)

		col := hash % uint64(c.m)
		val := c.table[i][col]
		if first || val < min {
			min = val
			first = false
		}
	}
	return min
}

func (c *CountMinSketch) Merge(other *CountMinSketch) error {
	if c.k != other.k {
		return fmt.Errorf("razlikuju se u broju redova")
	}
	if c.m != other.m {
		return fmt.Errorf("ne poklapaju se u broju kolona")
	}

	for i := uint(0); i < c.k; i++ {
		for j := uint(0); j < c.m; j++ {
			c.table[i][j] += other.table[i][j]
		}
	}
	return nil
}

func (c *CountMinSketch) Clear() {
	for i := uint(0); i < c.k; i++ {
		for j := uint(0); j < c.m; j++ {
			c.table[i][j] = 0
		}
	}
}

// OVAJ DEO BRACO SA JSON, TO NE SLUZI PROJEKTU AL SLUZI ZA TESTIRANJE
// DA NE MORAMO DA DESIFRUJEMO BINARNI FAJL NEGO OTVORIS JSON

type CountMinSketchJSON struct {
	K          uint     `json:"k"`
	M          uint     `json:"m"`
	Confidence float64  `json:"confidence"`
	Accuracy   float64  `json:"accuracy"`
	Table      [][]uint `json:"table"`
	Seeds      [][]byte `json:"seeds"`
}

func (c *CountMinSketch) MarshalJSON() ([]byte, error) {
	seeds := make([][]byte, len(c.hashFunctions))
	for i, v := range c.hashFunctions {
		seeds[i] = v.Seed
	}

	j := CountMinSketchJSON{
		K:          c.k,
		M:          c.m,
		Confidence: c.confidence,
		Accuracy:   c.accuracy,
		Table:      c.table,
		Seeds:      seeds,
	}
	return json.Marshal(j)
}

func (c *CountMinSketch) UnmarshalJSON(data []byte) error {
	var j CountMinSketchJSON
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	c.k = j.K
	c.m = j.M
	c.confidence = j.Confidence
	c.accuracy = j.Accuracy
	c.table = j.Table
	c.hashFunctions = CreateHashFunctions(j.Seeds)

	return nil
}

func (c *CountMinSketch) WriteToJSON(path string) error {
	data, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *CountMinSketch) ReadFromJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

// ZNACI BRACO OVO SE KORISTI KAD TREBA DA SE ZAPISUJE U OSTALE STVARI U PROJEKTU
// ZATO NIGDE NIJE DAT WRITER ILI GDE SE ZAPISUJE
// ZAPISUJE UNIVERZALNO U STA GOD

func (c *CountMinSketch) WriteTo(writer io.Writer) (int64, error) {
	var err error
	var written int64
	err = binary.Write(writer, binary.BigEndian, uint64(c.k))
	if err != nil {
		return written, err
	}
	written += 8

	err = binary.Write(writer, binary.BigEndian, uint64(c.m))
	if err != nil {
		return written, err
	}
	written += 8

	err = binary.Write(writer, binary.BigEndian, c.accuracy)
	if err != nil {
		return written, err
	}
	written += 8

	err = binary.Write(writer, binary.BigEndian, c.confidence)
	if err != nil {
		return written, err
	}
	written += 8

	// OVO JE BRACO ZA SEEDove
	err = binary.Write(writer, binary.BigEndian, uint64(len(c.hashFunctions)))
	if err != nil {
		return written, err
	}
	written += 8

	for _, v := range c.hashFunctions {
		err = binary.Write(writer, binary.BigEndian, uint64(len(v.Seed)))
		if err != nil {
			return written, err
		}
		written += 8

		err = binary.Write(writer, binary.BigEndian, v.Seed)
		if err != nil {
			return written, err
		}
		written += int64(len(v.Seed))

	}

	// table ZAPISIVANJE
	for i := uint(0); i < c.k; i++ {
		for j := uint(0); j < c.m; j++ {
			err = binary.Write(writer, binary.BigEndian, uint64(c.table[i][j]))
			if err != nil {
				return written, err
			}
			written += 8
		}
	}

	return written, nil
}

func (c *CountMinSketch) ReadFrom(reader io.Reader) (int64, error) {
	var read int64
	var err error

	var K uint64
	err = binary.Read(reader, binary.BigEndian, &K)
	if err != nil {
		return read, err
	}
	read += 8
	c.k = uint(K)

	var M uint64
	err = binary.Read(reader, binary.BigEndian, &M)
	if err != nil {
		return read, err
	}
	read += 8
	c.m = uint(M)

	var Accuracy float64
	err = binary.Read(reader, binary.BigEndian, &Accuracy)
	if err != nil {
		return read, err
	}
	read += 8
	c.accuracy = Accuracy

	var Confidence float64
	err = binary.Read(reader, binary.BigEndian, &Confidence)
	if err != nil {
		return read, err
	}
	read += 8
	c.confidence = Confidence

	var brojSeedova uint64
	err = binary.Read(reader, binary.BigEndian, &brojSeedova)
	if err != nil {
		return read, err
	}
	read += 8
	c.hashFunctions = make([]HashWithSeed, brojSeedova)

	for i := uint(0); i < uint(brojSeedova); i++ {
		var duzinaSeeda uint64
		err = binary.Read(reader, binary.BigEndian, &duzinaSeeda)
		if err != nil {
			return read, err
		}
		read += 8
		seed := make([]byte, duzinaSeeda)
		n, err := io.ReadFull(reader, seed)
		if err != nil {
			return read, err
		}
		read += int64(n)
		c.hashFunctions[i] = HashWithSeed{
			Seed: seed,
		}
	}

	c.table = make([][]uint, c.k)
	for i := uint(0); i < c.k; i++ {
		c.table[i] = make([]uint, c.m)

		for j := uint(0); j < c.m; j++ {
			var vrednost uint64
			err = binary.Read(reader, binary.BigEndian, &vrednost)
			if err != nil {
				return read, err
			}
			read += 8
			c.table[i][j] = uint(vrednost)
		}
	}

	return read, nil
}
