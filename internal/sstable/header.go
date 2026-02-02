package sstable

import (
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type Header struct {
	HasCompression    bool
	DataOffset        uint64
	DataSize          uint64
	FilterOffset      uint64
	FilterSize        uint64
	IndexOffset       uint64
	IndexSize         uint64
	SummaryOffset     uint64
	SummarySize       uint64
	MetaDataOffset    uint64
	MetaDataSize      uint64
	CompressionOffset uint64
	CompressionSize   uint64
}

func NewHeader() *Header {
	return &Header{
		HasCompression:    false,
		DataOffset:        0,
		DataSize:          0,
		FilterOffset:      0,
		FilterSize:        0,
		IndexOffset:       0,
		IndexSize:         0,
		SummaryOffset:     0,
		SummarySize:       0,
		MetaDataOffset:    0,
		MetaDataSize:      0,
		CompressionOffset: 0,
		CompressionSize:   0,
	}
}

func (h *Header) WriteHeader(file *os.File, header *Header) error {
	_, err := file.Seek(0, 0)
	if err != nil {
		return err
	}
	if header.HasCompression {
		_, err = file.Write([]byte{1})
		if err != nil {
			return err
		}
	} else {
		_, err = file.Write([]byte{0})
		if err != nil {
			return err
		}
	}
	err = utils.WriteUvarint(file, header.DataOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.DataSize)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.FilterOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.FilterSize)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.IndexOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.IndexSize)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.SummaryOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.SummarySize)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.MetaDataOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.MetaDataSize)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.CompressionOffset)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, header.CompressionSize)
	if err != nil {
		return err
	}
	return nil
}

func (h *Header) readHeader(file *os.File) (*Header, error) {
	_, err := file.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	header := &Header{}
	buf := make([]byte, 1)
	r, err := file.Read(buf)
	if r != 1 {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	hasComp := buf[0]
	hasCompression := false
	if hasComp == 1 {
		hasCompression = true
	}
	header.HasCompression = hasCompression
	dataOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	dataSize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.DataOffset = dataOffset
	header.DataSize = dataSize
	filterOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	filterSize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.FilterOffset = filterOffset
	header.FilterSize = filterSize
	indexOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	indexSize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.IndexOffset = indexOffset
	header.IndexSize = indexSize
	summaryOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	summarySize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.SummaryOffset = summaryOffset
	header.SummarySize = summarySize
	metaDataOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	metaDataSize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.MetaDataOffset = metaDataOffset
	header.MetaDataSize = metaDataSize
	compressionOffset, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	compressionSize, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	header.CompressionOffset = compressionOffset
	header.CompressionSize = compressionSize
	return header, nil
}
