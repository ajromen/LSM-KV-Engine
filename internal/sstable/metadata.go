package sstable

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// MerkleNode REPRESENTS A SINGLE NODE IN MERKLE TREE
type MerkleNode struct {
	Hash       [32]byte    // sha256 hash of this node
	LeftChild  *MerkleNode // pointer to the left child
	RightChild *MerkleNode // pointer to the right child
	IsLeaf     bool        // is node leaf
}

// MerkleTree REPRESENTS A MERKLE TREE FOR DATA BLOCK VERIFICATION
type MerkleTree struct {
	Root          *MerkleNode   // root of the merkle tree
	LeafNodes     []*MerkleNode // leaf nodes of the merkle tree
	NumDataBlocks uint32        // number of data blocks hashed and written into tree
}

func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		Root:          nil,
		LeafNodes:     make([]*MerkleNode, 0),
		NumDataBlocks: 0,
	}
}

// AddLeaf ADDS A NEW LEAF NODE REPRESENTING ONE DATA BLOCK HASH
func (mt *MerkleTree) AddLeaf(blockHash [32]byte) {
	leaf := &MerkleNode{
		Hash:       blockHash,
		IsLeaf:     true,
		LeftChild:  nil,
		RightChild: nil,
	}
	mt.LeafNodes = append(mt.LeafNodes, leaf)
	mt.NumDataBlocks++
}

// Build CONSTRUCTS MERKLE TREE FROM GIVEN LEAF NODES UP TO THE ROOT
func (mt *MerkleTree) Build() error {
	if len(mt.LeafNodes) == 0 {
		return errors.New("no leaves to begin with")
	}

	// start from leaf level
	currentLevel := make([]*MerkleNode, len(mt.LeafNodes))
	copy(currentLevel, mt.LeafNodes)

	// build levels until only root remains
	for len(currentLevel) > 1 {

		// initialize next level
		nextLevel := make([]*MerkleNode, 0)
		for i := 0; i < len(currentLevel); i += 2 {
			var newNode *MerkleNode

			// combine left and right child hashes into one singular hashes if pair exists
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
				// duplicate the last node if a node doesn't have its pair -> bitcoin method
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

	mt.Root = currentLevel[0]
	return nil
}

func (mt *MerkleTree) GetRootHash() [32]byte {
	if mt.Root == nil {
		return [32]byte{}
	}
	return mt.Root.Hash
}

// HashDataBlock hashes raw data block bytes using sha256 hashing
func HashDataBlock(blockData []byte) [32]byte {
	return sha256.Sum256(blockData)
}

// encodedSize RETURNS THE NUMBER OF BYTES NEEDED TO ENCODE THIS MERKLE TREE
func (mt *MerkleTree) encodedSize() int {
	size := 4                      // uint32 -> number of data blocks
	size += 4                      // uint32 -> number of leaf nodes
	size += 32 * len(mt.LeafNodes) // 32byte hash of every data block (leaves)
	size += 32                     // root
	size += 4                      // crc
	return size
}

// Encode SERIALIZES MERKLE TREE INTO A BYTE BUFFER
func (mt *MerkleTree) Encode() []byte {
	size := mt.encodedSize()
	buf := make([]byte, size)
	pos := 0

	// write number of data blocks
	binary.LittleEndian.PutUint32(buf[pos:], mt.NumDataBlocks)
	pos += 4

	// write number of tree nodes
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(mt.LeafNodes)))
	pos += 4

	// write all the leaf node hashes
	for _, leaf := range mt.LeafNodes {
		copy(buf[pos:pos+32], leaf.Hash[0:32])
		pos += 32
	}

	// write the root
	if mt.Root != nil {
		copy(buf[pos:pos+32], mt.Root.Hash[0:32])
		pos += 32
	} else {
		pos += 32
	}

	// calculate and write crc
	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)
	return buf
}

// Decode DESERIALIZES A MERKLE TREE FROM BYTE BUFFER
func (mt *MerkleTree) Decode(buf []byte) (*MerkleTree, error) {
	if len(buf) < 4+4+32+4 { // minimal: numBlocks + leafCount + root + crc
		return nil, errors.New("buffer too small")
	}

	// validate crc
	crcPos := len(buf) - 4
	expectedCRC := binary.LittleEndian.Uint32(buf[crcPos : crcPos+4])
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	if crc != expectedCRC {
		return nil, errors.New("CRC mismatch")
	}

	// read number of data blocks
	pos := 0
	numDataBlocks := binary.LittleEndian.Uint32(buf[pos : pos+4])

	// read number of leaf nodes
	pos += 4
	lenLeafNodes := binary.LittleEndian.Uint32(buf[pos : pos+4])

	pos += 4
	mt = &MerkleTree{
		LeafNodes:     make([]*MerkleNode, lenLeafNodes),
		NumDataBlocks: numDataBlocks,
	}

	// read all the hashes of leaf nodes
	for i := uint32(0); i < lenLeafNodes; i++ {
		if pos+32 > crcPos {
			return nil, errors.New("buffer too small for leaf hashes")
		}
		var hash [32]byte
		copy(hash[:], buf[pos:pos+32])
		pos += 32
		mt.LeafNodes[i] = &MerkleNode{
			Hash:   hash,
			IsLeaf: true,
		}
	}

	// read the root hash
	if pos+32 > crcPos {
		return nil, errors.New("buffer too small for root hash")
	}
	var rootHash [32]byte
	copy(rootHash[:], buf[pos:pos+32])
	pos += 32

	// build a tree from leaf nodes and compare with rootHash that has been read
	if len(mt.LeafNodes) > 0 {
		if err := mt.Build(); err != nil {
			return nil, err
		}
		if mt.Root.Hash != rootHash {
			return nil, errors.New("root hash mismatch after rebuild")
		}
	}
	return mt, nil
}

// ValidationResult CONTAINS RESULT OF THE MERKLE TREE VALIDATION
type ValidationResult struct {
	Valid            bool                // true -> root hashes match
	CorruptedBlocks  []uint32            // index of all corrupted data blocks
	ExpectedRootHash [32]byte            // expected root hash
	ActualRootHash   [32]byte            // actual root hash computed from given data
	BlockHashes      map[uint32][32]byte // hashes of corrupted blocks
}

// Verify BUILDS A MERKLE TREE FROM GIVEN BLOCK HASHES AND COMPARES IT WITH EXPECTED MERKLE TREE
func (mt *MerkleTree) Verify(blockHashes [][32]byte) (*ValidationResult, error) {
	if len(blockHashes) == 0 {
		return &ValidationResult{}, errors.New("block hashes is empty")
	}
	if len(blockHashes) != len(mt.LeafNodes) {
		return &ValidationResult{Valid: false}, errors.New("block hashes length mismatch")
	}
	if uint32(len(blockHashes)) != mt.NumDataBlocks {
		return &ValidationResult{Valid: false}, errors.New("block hashes count != NumDataBlocks")
	}
	result := &ValidationResult{
		Valid:            true,
		CorruptedBlocks:  make([]uint32, 0),
		ExpectedRootHash: mt.GetRootHash(),
		BlockHashes:      make(map[uint32][32]byte),
	}
	// build a tree from given block hashes
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
	// compare hashes
	if mt.GetRootHash() == actualTree.GetRootHash() {
		result.Valid = true
		return result, nil
	}
	// if hashes not equal find corrupted blocks
	result.Valid = false
	corruptedBlocks := findCorruptedBlocks(mt.Root, actualTree.Root, 0, len(mt.LeafNodes))
	result.CorruptedBlocks = corruptedBlocks
	for _, blockIdx := range corruptedBlocks {
		result.BlockHashes[blockIdx] = blockHashes[blockIdx]
	}
	return result, nil
}

// findCorruptedBlocks RECURSIVELY COMPARES TWO MERKLE SUBTREES AND RETURNS INDEXES OF LEAF NODES WHOSE HASHES ARE DIFFERENT
func findCorruptedBlocks(expectedNode, actualNode *MerkleNode, startIdx, endIdx int) []uint32 {
	// if node that is being compared is leaf no need for recursion
	if expectedNode.IsLeaf {
		if expectedNode.Hash != actualNode.Hash {
			return []uint32{uint32(startIdx)}
		}
		return nil
	}
	if expectedNode.Hash == actualNode.Hash {
		return nil
	}
	// if node that is being compared is not leaf recursively compare two merkle subtrees
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

// MerkleProof REPRESENTS A MERKLE INCLUSION PROOF FOR A SINGLE DATA BLOCK

type MerkleProof struct {
	BlockIndex uint32     // index of the data block in sstable for which this proof is constructed
	BlockHash  [32]byte   // hash of given data block
	Proof      [][32]byte // ordered list of sibling hashes from the leaf level up to the root.
	RootHash   [32]byte   // expected merkle root hash of the tree the block is claimed to belong to
}

// GetProof BUILDS A MERKLE PROOF FOR THE DATA BLOCK AT BLOCKINDEX
// NOTE : The proof can later be verified using VerifyProof without rebuilding the entire tree.
func (mt *MerkleTree) GetProof(blockIndex uint32) (*MerkleProof, error) {
	if blockIndex >= uint32(len(mt.LeafNodes)) {
		return nil, errors.New("block index out of range")
	}
	if mt.Root == nil {
		return nil, errors.New("merkle tree not built")
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
		if siblingIndex >= len(currentLevel) {
			siblingIndex = index
		}
		proof.Proof = append(proof.Proof, currentLevel[siblingIndex].Hash)
		nextLevel := make([]*MerkleNode, 0, (len(currentLevel)+1)/2)
		for i := 0; i < len(currentLevel); i += 2 {
			var left = currentLevel[i]
			var right *MerkleNode

			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				right = currentLevel[i]
			}
			combined := make([]byte, 64)
			copy(combined[0:32], left.Hash[:])
			copy(combined[32:64], right.Hash[:])
			newHash := sha256.Sum256(combined)
			nextLevel = append(nextLevel, &MerkleNode{
				Hash:       newHash,
				LeftChild:  left,
				RightChild: right,
				IsLeaf:     false,
			})
		}
		currentLevel = nextLevel
		index = index / 2
	}
	return proof, nil
}

// VerifyProof VERIFIES A MERKLE INCLUSION PROOF WITHOUT HAVING A FULL TREE
// -> IT RECOMPUTES THE ROOT HASH BY HASHING THE BLOCK HASH WITH ALL SIBLING HASHES
func VerifyProof(proof *MerkleProof) bool {
	if proof == nil || len(proof.Proof) == 0 {
		return false
	}
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
