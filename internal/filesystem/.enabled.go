package filesystem

// Read returns bytes from a file
func (fs *Fuse) Read(path string, buff []byte, ofst int64, fh uint64) int {

	n.node.stat.Atim = Now()

	return copy(buff, data)
}
