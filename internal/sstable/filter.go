package sstable

import "github.com/ajromen/LSM-KV-Engine/internal/probabilistics"

type FilterSegment struct {
	filter *probabilistics.BloomFilter
}

func (segment *FilterSegment) Filter() *probabilistics.BloomFilter {
	return segment.filter
}
