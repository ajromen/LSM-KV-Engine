package sstable

type SummaryEntry struct {
	Key           []byte //first key of index block
	Offset        uint64 //offset of the index block
	NumEntries    uint32 //number of entries in the index block
	MinDataOffset uint64 //minimum data block offset in the index block
	MaxDataOffset uint64 //maximum data block offset in the index block
}

type SummarySegment struct {
	MinKey           []byte         //min key (border of index file)
	Entries          []SummaryEntry //entries (first record of every samplingdegree-nth block
	MaxKey           []byte         //max key (rigiht border of index file)
	SamplingDegree   uint32         //for entries pise
	TotalIndexBlocks uint32         //number of index blocks -> maybe need for footer
}
