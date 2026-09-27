package vault

// maxAncestry bounds the revisions a container names before its own; the 63 beyond the previous take at most 2,144 index bytes.
const maxAncestry = 64

// ancestry holds the hashes of the revisions before a container's own: ancestry[i] is the hash of revision Revision-1-i.
type ancestry [][32]byte

// after is the ancestry of the revision following the one with hash; the oldest hash drops at maxAncestry.
func (a ancestry) after(hash [32]byte) ancestry {
	return append(ancestry{hash}, a[:min(len(a), maxAncestry-1)]...)
}

// names reports whether the container at revision descends from the earlier version witness names.
func (a ancestry) names(revision uint64, witness Witness) bool {
	if witness.Revision >= revision {
		return false
	}
	back := revision - 1 - witness.Revision
	return back < uint64(len(a)) && a[back] == witness.Hash
}

// previous is the index's previous field: the hash of the revision before, zero for the first.
func (a ancestry) previous() [32]byte {
	if len(a) == 0 {
		return [32]byte{}
	}
	return a[0]
}

// earlier is the index's earlier field: the hashes before the previous one.
func (a ancestry) earlier() [][32]byte {
	if len(a) < 2 {
		return nil
	}
	return a[1:]
}
