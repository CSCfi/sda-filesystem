package filesystem

/*type mockConnectfs struct {
	fuseParent
	mock.Mock
}

func getMockFuse(t *testing.T) (fs *mockConnectfs) {
	var nodes jsonNode
	if err := json.Unmarshal([]byte(testFuse), &nodes); err != nil {
		t.Fatal("Could not unmarshal json")
	}

	fs = &mockConnectfs{}
	fs.root = &node{}
	fs.root.stat.Mode = fuse.S_IFDIR | sRDONLY
	fs.root.stat.Size = nodes.Size
	fs.root.chld = map[string]*node{}
	fs.renamed = map[string]string{}

	assignChildren(fs.root, nodes.Children)
	return
}

func (fs *mockConnectfs) openNode(path string, dir bool) (*node, int, uint64) {
	args := fs.Called(path, dir)
	return args.Get(0).(*node), args.Int(1), args.Get(2).(uint64)
}

func TestOpen(t *testing.T) {
	fs := getMockFuse(t)

	var tests = []struct {
		mockSpecialHeaders         func(path string) (bool, int64, error)
		mockCalculateDecryptedSize func(size int64) int64
		node                       *node
		size                       int64
		fh                         uint64
		errc                       int
		checkDecryption            bool
		path, testname             string
	}{
		{
			func(path string) (bool, int64, error) {
				return false, 0, nil
			},
			func(size int64) int64 {
				return 0
			},
			fs.root.chld["child_1"],
			fs.root.chld["child_1"].stat.Size,
			^uint64(0), -fuse.EAGAIN,
			true,
			"child_1", "API_ERR",
		},
	}

	origGetSpecialHeaders := api.GetSpecialHeaders
	origCalculateDecryptedSize := calculateDecryptedSize

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetSpecialHeaders = tt.mockSpecialHeaders
			calculateDecryptedSize = tt.mockCalculateDecryptedSize

			fs.On("openNode", mock.Anything).Return(tt.node, tt.errc, tt.fh).Once()

			errc, fh := fs.Open(tt.path, 0)
			if errc != tt.errc {
				t.Errorf("Error code incorrect. Expected %d, got %d", tt.errc, errc)
			} else if fh != tt.fh {
				t.Errorf("File handle incorrect. Expected %d, got %d", tt.fh, fh)
			}
		})
	}

	api.GetSpecialHeaders = origGetSpecialHeaders
	calculateDecryptedSize = origCalculateDecryptedSize
}

func TestRead(t *testing.T) {
}*/
