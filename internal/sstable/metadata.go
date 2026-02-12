package sstable

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

type MerkleNode struct {
	Hash       [32]byte
	LeftChild  *MerkleNode
	RightChild *MerkleNode
	IsLeaf     bool
}

type MerkleTree struct {
	Root          *MerkleNode
	LeafNodes     []*MerkleNode
	NumDataBlocks uint32
}

func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		Root:          nil,
		LeafNodes:     make([]*MerkleNode, 0),
		NumDataBlocks: 0,
	}
}

func (tree *MerkleTree) AddLeaf(blockHash [32]byte) {
	leaf := &MerkleNode{
		Hash:       blockHash,
		IsLeaf:     true,
		LeftChild:  nil,
		RightChild: nil,
	}
	tree.LeafNodes = append(tree.LeafNodes, leaf)
	tree.NumDataBlocks++
}

func (tree *MerkleTree) Build() error {
	if len(tree.LeafNodes) == 0 {
		return errors.New("no leaves to begin with")
	}
	currentLevel := make([]*MerkleNode, len(tree.LeafNodes))
	copy(currentLevel, tree.LeafNodes)
	for len(currentLevel) > 1 {
		nextLevel := make([]*MerkleNode, 0)
		for i := 0; i < len(currentLevel); i += 2 {
			var newNode *MerkleNode
			if i+1 < len(currentLevel) {
				combined := make([]byte, 64)
				copy(combined[0:32], currentLevel[i].Hash[0:32])
				copy(combined[32:64], currentLevel[i+1].Hash[0:32])
				newHash := sha256.Sum256(combined)
				newNode = &MerkleNode{
					Hash:       newHash,
					LeftChild:  currentLevel[i],
					RightChild: currentLevel[i+1],
					IsLeaf:     false,
				}
			} else {
				combined := make([]byte, 64)
				copy(combined[0:32], currentLevel[i].Hash[0:32])
				copy(combined[32:64], currentLevel[i].Hash[0:32])
				newHash := sha256.Sum256(combined)
				newNode = &MerkleNode{
					Hash:       newHash,
					LeftChild:  currentLevel[i],
					RightChild: currentLevel[i],
					IsLeaf:     false,
				}
			}
			nextLevel = append(nextLevel, newNode)
		}
		currentLevel = nextLevel
	}
	tree.Root = currentLevel[0]
	return nil
}

func (tree *MerkleTree) GetRootHash() [32]byte {
	if tree.Root == nil {
		return [32]byte{}
	}
	return tree.Root.Hash
}

func HashDataBlock(blockData []byte) [32]byte {
	return sha256.Sum256(blockData)
}

func (mt *MerkleTree) encodedSize() int {
	size := 4                      // uint32 -> number of data blocks
	size += 4                      // uint32 -> number of leaf nodes
	size += 32 * len(mt.LeafNodes) // 32byte hash of every data block (leaves)
	size += 32                     // root
	size += 4                      // crc
	return size
}

func (tree *MerkleTree) Encode() []byte {
	size := tree.encodedSize()
	buf := make([]byte, size)
	pos := 0
	binary.LittleEndian.PutUint32(buf[pos:], tree.NumDataBlocks)
	pos += 4
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(tree.LeafNodes)))
	pos += 4
	for _, leaf := range tree.LeafNodes {
		copy(buf[pos:pos+32], leaf.Hash[0:32])
		pos += 32
	}
	if tree.Root != nil {
		copy(buf[pos:pos+32], tree.Root.Hash[0:32])
		pos += 32
	} else {
		pos += 32
	}
	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)
	return buf
}

func (tree *MerkleTree) Decode(buf []byte) (*MerkleTree, error) {
	crcPos := len(buf) - 4
	expectedCRC := binary.LittleEndian.Uint32(buf[crcPos : crcPos+4])
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	if crc != expectedCRC {
		return nil, errors.New("CRC mismatch")
	}
	pos := 0
	numDataBlocks := binary.LittleEndian.Uint32(buf[pos : pos+4])
	pos += 4
	lenLeafNodes := binary.LittleEndian.Uint32(buf[pos : pos+4])
	pos += 4
	tree = &MerkleTree{
		LeafNodes:     make([]*MerkleNode, lenLeafNodes),
		NumDataBlocks: numDataBlocks,
	}
	for i := uint32(0); i < lenLeafNodes; i++ {
		if pos+32 > crcPos {
			return nil, errors.New("buffer too small for leaf hashes")
		}
		var hash [32]byte
		copy(hash[:], buf[pos:pos+32])
		pos += 32
		tree.LeafNodes[i] = &MerkleNode{
			Hash:   hash,
			IsLeaf: true,
		}
	}
	if pos+32 > crcPos {
		return nil, errors.New("buffer too small for root hash")
	}
	var rootHash [32]byte
	copy(rootHash[:], buf[pos:pos+32])
	pos += 32
	if len(tree.LeafNodes) > 0 {
		if err := tree.Build(); err != nil {
			return nil, err
		}
		if tree.Root.Hash != rootHash {
			return nil, errors.New("root hash mismatch after rebuild")
		}
	}
	return tree, nil
}

type ValidationResult struct {
	Valid            bool
	CorruptedBlocks  []uint32
	ExpectedRootHash [32]byte
	ActualRootHash   [32]byte
	BlockHashes      map[uint32][32]byte
}

func (tree *MerkleTree) Verify(blockHashes [][32]byte) (*ValidationResult, error) {
	if len(blockHashes) == 0 {
		return &ValidationResult{}, errors.New("block hashes is empty")
	}
	if len(blockHashes) != len(tree.LeafNodes) {
		return &ValidationResult{Valid: false}, errors.New("block hashes length mismatch")
	}
	result := &ValidationResult{
		Valid:            true,
		CorruptedBlocks:  make([]uint32, 0),
		ExpectedRootHash: tree.GetRootHash(), // Raw bytes
		BlockHashes:      make(map[uint32][32]byte),
	}
	actualTree := NewMerkleTree()
	for _, blockHash := range blockHashes {
		actualTree.AddLeaf(blockHash)
	}
	if err := actualTree.Build(); err != nil {
		return &ValidationResult{}, err
	}
	if actualTree.Root == nil {
		return &ValidationResult{}, errors.New("no root")
	}
	result.ActualRootHash = actualTree.GetRootHash()
	if tree.GetRootHash() == actualTree.GetRootHash() {
		result.Valid = true
		return result, nil
	}
	result.Valid = false
	corruptedBlocks := findCorruptedBlocks(tree.Root, actualTree.Root, 0, len(tree.LeafNodes))
	result.CorruptedBlocks = corruptedBlocks
	for _, blockIdx := range corruptedBlocks {
		result.BlockHashes[blockIdx] = blockHashes[blockIdx]
	}
	return result, nil
}

func findCorruptedBlocks(expectedNode, actualNode *MerkleNode, startIdx, endIdx int) []uint32 {
	if expectedNode.IsLeaf {
		if expectedNode.Hash != actualNode.Hash {
			return []uint32{uint32(startIdx)}
		}
		return nil
	}
	if expectedNode.Hash == actualNode.Hash {
		return nil
	}
	corrupted := make([]uint32, 0)
	mid := startIdx + (endIdx-startIdx)/2
	if expectedNode.LeftChild != nil && actualNode.LeftChild != nil {
		leftCorrupted := findCorruptedBlocks(expectedNode.LeftChild, actualNode.LeftChild, startIdx, mid)
		corrupted = append(corrupted, leftCorrupted...)
	}
	if expectedNode.RightChild != nil && actualNode.RightChild != nil {
		rightStart := mid
		if endIdx > mid {
			rightCorrupted := findCorruptedBlocks(expectedNode.RightChild, actualNode.RightChild, rightStart, endIdx)
			corrupted = append(corrupted, rightCorrupted...)
		}
	}
	return corrupted
}

type MerkleProof struct {
	BlockIndex uint32
	BlockHash  [32]byte
	Proof      [][32]byte // Raw bytes!
	RootHash   [32]byte
}

func (mt *MerkleTree) GetProof(blockIndex uint32) (*MerkleProof, error) {
	if blockIndex >= uint32(len(mt.LeafNodes)) {
		return nil, errors.New("block index out of range")
	}
	proof := &MerkleProof{
		BlockIndex: blockIndex,
		BlockHash:  mt.LeafNodes[blockIndex].Hash,
		Proof:      make([][32]byte, 0),
		RootHash:   mt.GetRootHash(),
	}
	currentLevel := make([]*MerkleNode, len(mt.LeafNodes))
	copy(currentLevel, mt.LeafNodes)
	index := int(blockIndex)
	for len(currentLevel) > 1 {
		var siblingIndex int
		if index%2 == 0 {
			siblingIndex = index + 1
		} else {
			siblingIndex = index - 1
		}
		if siblingIndex < len(currentLevel) {
			proof.Proof = append(proof.Proof, currentLevel[siblingIndex].Hash)
		}
		nextLevel := make([]*MerkleNode, 0)
		for i := 0; i < len(currentLevel); i += 2 {
			var newNode *MerkleNode
			if i+1 < len(currentLevel) {
				combined := make([]byte, 64)
				copy(combined[0:32], currentLevel[i].Hash[0:32])
				copy(combined[32:64], currentLevel[i+1].Hash[0:32])
				newHash := sha256.Sum256(combined)
				newNode = &MerkleNode{
					Hash: newHash,
				}
			} else {
				newNode = &MerkleNode{
					Hash: currentLevel[i].Hash,
				}
			}
			nextLevel = append(nextLevel, newNode)
		}
		currentLevel = nextLevel
		index = index / 2
	}
	return proof, nil
}

func VerifyProof(proof *MerkleProof) bool {
	currentHash := proof.BlockHash
	index := proof.BlockIndex
	for _, siblingHash := range proof.Proof {
		if index%2 == 0 {
			combined := make([]byte, 64)
			copy(combined[0:32], currentHash[0:32])
			copy(combined[32:64], siblingHash[0:32])
			currentHash = sha256.Sum256(combined)
		} else {
			combined := make([]byte, 64)
			copy(combined[0:32], siblingHash[0:32])
			copy(combined[32:64], currentHash[0:32])
			currentHash = sha256.Sum256(combined)
		}
		index = index / 2
	}
	return currentHash == proof.RootHash
}

func DecodeMerkleTree(buf []byte) (*MerkleTree, error) {
	crcPos := len(buf) - 4
	expectedCRC := binary.LittleEndian.Uint32(buf[crcPos : crcPos+4])
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	if crc != expectedCRC {
		return nil, errors.New("CRC mismatch")
	}
	pos := 0
	numDataBlocks := binary.LittleEndian.Uint32(buf[pos : pos+4])
	pos += 4
	lenLeafNodes := binary.LittleEndian.Uint32(buf[pos : pos+4])
	pos += 4
	tree = &MerkleTree{
		LeafNodes:     make([]*MerkleNode, lenLeafNodes),
		NumDataBlocks: numDataBlocks,
	}
	for i := uint32(0); i < lenLeafNodes; i++ {
		if pos+32 > crcPos {
			return nil, errors.New("buffer too small for leaf hashes")
		}
		var hash [32]byte
		copy(hash[:], buf[pos:pos+32])
		pos += 32
		tree.LeafNodes[i] = &MerkleNode{
			Hash:   hash,
			IsLeaf: true,
		}
	}
	if pos+32 > crcPos {
		return nil, errors.New("buffer too small for root hash")
	}
	var rootHash [32]byte
	copy(rootHash[:], buf[pos:pos+32])
	pos += 32
	if len(tree.LeafNodes) > 0 {
		if err := tree.Build(); err != nil {
			return nil, err
		}
		if tree.Root.Hash != rootHash {
			return nil, errors.New("root hash mismatch after rebuild")
		}
	}
	return tree, nil
}
