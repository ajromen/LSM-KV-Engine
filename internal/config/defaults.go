package config

// Za sva podešavanja koja nedostaju u konfiguracionom fajlu sistem treba da dodeli
//default vrednosti koje se navode u kodu

// Engine defaults
const (
	//WAL
	DefaultWalSegmentSize = 1 * 1024 * 1024

	//memtable
	MemtableType              = "hashmap"
	DefaultMemtableMaxEntries = 1000

	//SSTable
	DefaultSSTableBlockSize = 16

	//SkipList
	DefaultSkipListMaxLevel = 16
)

func NewDefaultConfig() *Config {
	return &Config{
		WAL: WALConfig{
			WALSegmentSize: DefaultWalSegmentSize,
		},
		Memtable: MemtableConfig{
			MemtableType:    MemtableType,
			MemtableMaxSize: DefaultMemtableMaxEntries,
		},
		SSTable: SSTableConfig{
			SSTableDataBlockSize: DefaultSSTableBlockSize,
		},
		SkipList: SkipListConfig{
			MaxLevel: DefaultSkipListMaxLevel,
		},
	}
}

//func applyDefaults(cfg *Config) {
//	if cfg.WAL.WALSegmentSize == 0 {
//		cfg.WAL.WALSegmentSize = 1024 * 1024
//	}
//	// Ovde kaze "Maksimalnu velicinu specificira korisnik tako sto navodi broj elemenata ILI zauzece memorije u KB" - na
//	// nama je da vidimo hocemo li birati jedno od ta dva ili proveravati koji je popunjen i dati koristiti
//	if cfg.Memtable.MemtableMaxSize == 0 && cfg.Memtable.MemtableSizeKB == 0 {
//		cfg.Memtable.MemtableMaxSize = 1000
//	}
//	if cfg.Memtable.MemtableType == "" {
//		cfg.Memtable.MemtableType = "hashmap"
//	}
//	if cfg.SSTable.SSTableDataBlockSize == 0 {
//		cfg.SSTable.SSTableDataBlockSize = 16 // Ovo je u KB (Tako je na LevelDB pa kontam da je ok)
//	}
//	if cfg.SkipList.MaxLevel == 0 {
//		cfg.SkipList.MaxLevel = 16
//	}
//}
