package sstable

import (
	"testing"
)

func TestMerkleTreeBuildAndRoot(t *testing.T) {
	t.Log("---- MERKLE TREE BUILD TEST ----")
	tree := NewMerkleTree()
	data1 := []byte("block1")
	data2 := []byte("block2")
	data3 := []byte("block3")
	tree.AddLeaf(HashDataBlock(data1))
	tree.AddLeaf(HashDataBlock(data2))
	tree.AddLeaf(HashDataBlock(data3))
	if err := tree.Build(); err != nil {
		t.Fatalf("failed to build merkle tree: %v", err)
	}
	if tree.Root == nil {
		t.Fatalf("expected root to be set")
	}
	root1 := tree.GetRootHash()
	tree2 := NewMerkleTree()
	tree2.AddLeaf(HashDataBlock(data1))
	tree2.AddLeaf(HashDataBlock(data2))
	tree2.AddLeaf(HashDataBlock(data3))
	if err := tree2.Build(); err != nil {
		t.Fatalf("failed to build second tree: %v", err)
	}
	root2 := tree2.GetRootHash()
	if root1 != root2 {
		t.Fatalf("expected same root hash for identical trees")
	}
}

func TestMerkleEncodeDecode(t *testing.T) {
	t.Log("---- MERKLE TREE ENCODE/DECODE TEST ----")
	tree := NewMerkleTree()
	tree.AddLeaf(HashDataBlock([]byte("a")))
	tree.AddLeaf(HashDataBlock([]byte("b")))
	tree.AddLeaf(HashDataBlock([]byte("c")))
	if err := tree.Build(); err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}
	encoded := tree.Encode()
	decodedTree, err := tree.Decode(encoded)
	if err != nil {
		t.Fatalf("failed to decode tree: %v", err)
	}
	if decodedTree.GetRootHash() != tree.GetRootHash() {
		t.Fatalf("expected same root hash after decode")
	}
}

func TestMerkleVerifyValid(t *testing.T) {
	t.Log("---- MERKLE TREE VERIFY VALID TEST ----")
	tree := NewMerkleTree()
	blocks := [][]byte{
		[]byte("block1"),
		[]byte("block2"),
		[]byte("block3"),
	}
	hashes := make([][32]byte, 0)
	for _, b := range blocks {
		h := HashDataBlock(b)
		hashes = append(hashes, h)
		tree.AddLeaf(h)
	}
	if err := tree.Build(); err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}
	result, err := tree.Verify(hashes)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected tree to be valid")
	}
}

func TestMerkleVerifyCorruptedBlock(t *testing.T) {
	t.Log("---- MERKLE TREE VERIFY CORRUPTED BLOCK TEST ----")
	tree := NewMerkleTree()
	blocks := [][]byte{
		[]byte("block1"),
		[]byte("block2"),
		[]byte("block3"),
	}
	hashes := make([][32]byte, 0)
	for _, b := range blocks {
		h := HashDataBlock(b)
		hashes = append(hashes, h)
		tree.AddLeaf(h)
	}
	if err := tree.Build(); err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}
	hashes[1] = HashDataBlock([]byte("corrupted"))
	result, err := tree.Verify(hashes)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected tree to be invalid due to corruption")
	}
	if len(result.CorruptedBlocks) == 0 {
		t.Fatalf("expected corrupted blocks to be reported")
	}
}

func TestMerkleProof(t *testing.T) {
	t.Log("---- MERKLE TREE PROOF TEST ----")
	tree := NewMerkleTree()
	blocks := [][]byte{
		[]byte("block1"),
		[]byte("block2"),
		[]byte("block3"),
		[]byte("block4"),
	}
	for _, b := range blocks {
		tree.AddLeaf(HashDataBlock(b))
	}
	if err := tree.Build(); err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}
	proof, err := tree.GetProof(2)
	if err != nil {
		t.Fatalf("failed to get proof: %v", err)
	}
	if proof.BlockIndex != 2 {
		t.Fatalf("expected block index 2, got %d", proof.BlockIndex)
	}
	if !VerifyProof(proof) {
		t.Fatalf("expected proof to be valid")
	}
	proof.BlockHash = HashDataBlock([]byte("fake"))
	if VerifyProof(proof) {
		t.Fatalf("expected tampered proof to be invalid")
	}
}
