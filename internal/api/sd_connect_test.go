package api

/*func TestCreateToken(t *testing.T) {
	var tests = []struct {
		username, password, token string
	}{
		{"user", "pass", "dXNlcjpwYXNz"},
		{"kalmari", "23t&io00_e", "a2FsbWFyaToyM3QmaW8wMF9l"},
		{"qwerty123", "mnbvc456", "cXdlcnR5MTIzOm1uYnZjNDU2"},
	}

	origToken := hi.ci.token
	defer func() { hi.ci.token = origToken }()

	for i, tt := range tests {
		testname := fmt.Sprintf("TOKEN_%d", i)
		t.Run(testname, func(t *testing.T) {
			hi.ci.token = ""
			CreateToken(tt.username, tt.password)
			if hi.ci.token != tt.token {
				t.Errorf("Username %q and password %q should have returned token %q, got %q",
					tt.username, tt.password, tt.token, hi.ci.token)
			}
		})
	}
}

func TestFetchTokens(t *testing.T) {
	var tests = []struct {
		mockGetUToken    func() error
		mockGetProjects  func() ([]Metadata, error)
		mockGetSToken    func(project string) error
		sTokens          map[string]SToken
		uToken, testname string
	}{
		{
			func() error {
				hi.ci.uToken = "uToken"
				return nil
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "project1", Bytes: 234}, {Name: "project2", Bytes: 52}, {Name: "project3", Bytes: 90}}, nil
			},
			func(project string) error {
				hi.ci.sTokens[project] = SToken{project + "_token", "435" + project}
				return nil
			},
			map[string]SToken{"project1": {"project1_token", "435project1"},
				"project2": {"project2_token", "435project2"},
				"project3": {"project3_token", "435project3"}},
			"uToken", "OK",
		},
		{
			func() error {
				hi.ci.uToken = "uToken"
				return errors.New("Error occurred")
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "project1", Bytes: 234}, {Name: "project2", Bytes: 52}, {Name: "project3", Bytes: 90}}, nil
			},
			func(project string) error {
				hi.ci.sTokens[project] = SToken{project + "_token", "435" + project}
				return nil
			},
			map[string]SToken{},
			"", "UTOKEN_ERROR",
		},
		{
			func() error {
				hi.ci.uToken = "new_token"
				return nil
			},
			func() ([]Metadata, error) {
				return nil, errors.New("Error")
			},
			func(project string) error {
				hi.ci.sTokens[project] = SToken{project + "_secret", "890" + project}
				return nil
			},
			map[string]SToken{},
			"new_token", "PROJECTS_ERROR",
		},
		{
			func() error {
				hi.ci.uToken = "another_token"
				return nil
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "pr1", Bytes: 43}, {Name: "pr2", Bytes: 51}, {Name: "pr3", Bytes: 900}}, nil
			},
			func(project string) error {
				if project == "pr2" {
					hi.ci.sTokens[project] = SToken{project + "_secret", "890" + project}
					return errors.New("New error")
				}
				hi.ci.sTokens[project] = SToken{"secret_token", "cactus"}
				return nil
			},
			map[string]SToken{"pr1": {"secret_token", "cactus"}, "pr3": {"secret_token", "cactus"}},
			"another_token", "STOKEN_ERROR",
		},
	}

	origGetUToken := GetUToken
	origGetProjects := GetProjects
	origGetSToken := GetSToken
	origUToken := hi.ci.uToken
	origSTokens := hi.ci.sTokens

	defer func() {
		GetUToken = origGetUToken
		GetProjects = origGetProjects
		GetSToken = origGetSToken
		hi.ci.uToken = origUToken
		hi.ci.sTokens = origSTokens
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			GetUToken = tt.mockGetUToken
			GetProjects = tt.mockGetProjects
			GetSToken = tt.mockGetSToken

			hi.ci.uToken = ""
			hi.ci.sTokens = map[string]SToken{}

			FetchTokens()

			if hi.ci.uToken != tt.uToken {
				t.Errorf("uToken incorrect. Expected %q, got %q", tt.uToken, hi.ci.uToken)
			} else if !reflect.DeepEqual(hi.ci.sTokens, tt.sTokens) {
				t.Errorf("sTokens incorrect.\nExpected %q\nGot %q", tt.sTokens, hi.ci.sTokens)
			}
		})
	}
}

func TestGetUToken(t *testing.T) {
	var tests = []struct {
		testname        string
		mockMakeRequest func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedToken   string
	}{
		{
			"FAIL",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting token")
			},
			"",
		},
		{
			"OK_1",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *UToken:
					*v = UToken{"myveryowntoken"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *UToken", reflect.TypeOf(v))
				}
			},
			"myveryowntoken",
		},
		{
			"OK_2",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *UToken:
					*v = UToken{"9765rty5678"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *UToken", reflect.TypeOf(v))
				}
			},
			"9765rty5678",
		},
	}

	origMakeRequest := makeRequest
	origUToken := hi.ci.uToken

	defer func() {
		makeRequest = origMakeRequest
		hi.ci.uToken = origUToken
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest
			hi.ci.uToken = ""

			err := GetUToken()

			if tt.expectedToken == "" {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedToken != hi.ci.uToken {
				t.Errorf("Unscoped token is incorrect. Expected %q, got %q", tt.expectedToken, hi.ci.uToken)
			}
		})
	}
}

func TestGetSToken(t *testing.T) {
	var tests = []struct {
		testname, project string
		mockMakeRequest   func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedToken     string
		expectedID        string
	}{
		{
			"FAIL", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting token")
			},
			"", "",
		},
		{
			"OK_1", "project007",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SToken:
					*v = SToken{"myveryowntoken", "jbowegxf72nfbof"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}
			},
			"myveryowntoken", "jbowegxf72nfbof",
		},
		{
			"OK_2", "projectID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SToken:
					*v = SToken{"9765rty5678", "ug8392nzdipqz9210z"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}
			},
			"9765rty5678", "ug8392nzdipqz9210z",
		},
	}

	origMakeRequest := makeRequest
	origSTokens := hi.ci.sTokens

	defer func() {
		makeRequest = origMakeRequest
		hi.ci.sTokens = origSTokens
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest
			hi.ci.sTokens = make(map[string]sToken)

			err := GetSToken(tt.project)

			if tt.expectedToken == "" {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if _, ok := hi.ci.sTokens[tt.project]; !ok {
				t.Errorf("Scoped token for %q is not defined", tt.project)
			} else if tt.expectedToken != hi.ci.sTokens[tt.project].Token {
				t.Errorf("Scoped token is incorrect. Expected %q, got %q", tt.expectedToken, hi.ci.sTokens[tt.project].Token)
			} else if tt.expectedID != hi.ci.sTokens[tt.project].ProjectID {
				t.Errorf("Project ID is incorrect. Expected %q, got %q", tt.expectedID, hi.ci.sTokens[tt.project].ProjectID)
			}
		})
	}
}

func TestGetProjects(t *testing.T) {
	var tests = []struct {
		testname         string
		mockMakeRequest  func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData []Metadata
	}{
		{
			"FAIL",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting projects")
			},
			nil,
		},
		{
			"EMPTY",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{234, "Jack"}, {2, "yur586bl"}, {7489, "rtu6u__78bgi"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{234, "Jack"}, {2, "yur586bl"}, {7489, "rtu6u__78bgi"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			projects, err := GetProjects()

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, projects) {
				t.Errorf("Projects incorrect. Expected %v, got %v", tt.expectedMetaData, projects)
			}
		})
	}
}

func TestGetContainers(t *testing.T) {
	var tests = []struct {
		testname, project string
		mockMakeRequest   func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData  []Metadata
	}{
		{
			"FAIL", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting containers")
			},
			nil,
		},
		{
			"EMPTY", "projectID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK", "project345",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{2341, "tukcdfku6"}, {45, "hf678cof7uib68or6"}, {6767, "rtu6u__78bgi"}, {1, "9ob89bio"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{2341, "tukcdfku6"}, {45, "hf678cof7uib68or6"}, {6767, "rtu6u__78bgi"}, {1, "9ob89bio"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			containers, err := GetContainers(tt.project)

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, containers) {
				t.Errorf("Containers incorrect. Expected %v, got %v", tt.expectedMetaData, containers)
			}
		})
	}
}

func TestGetObjects(t *testing.T) {
	var tests = []struct {
		testname, project, container string
		mockMakeRequest              func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData             []Metadata
	}{
		{
			"FAIL", "", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting containers")
			},
			nil,
		},
		{
			"EMPTY", "projectID", "container349",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK", "project345", "containerID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{56, "tukcdfku6"}, {5, "hf678cof7uib68or6"}, {47685, "rtu6u__78bgi"}, {10, "9ob89bio"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{56, "tukcdfku6"}, {5, "hf678cof7uib68or6"}, {47685, "rtu6u__78bgi"}, {10, "9ob89bio"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			objects, err := GetObjects(tt.project, tt.container)

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, objects) {
				t.Errorf("Objects incorrect. Expected %v, got %v", tt.expectedMetaData, objects)
			}
		})
	}
}

func TestGetSpecialHeaders(t *testing.T) {
	var tests = []struct {
		testname, path  string
		mockMakeRequest func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedHeaders SpecialHeaders
	}{
		{
			"FAIL", "project/container/object",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting headers")
			},
			SpecialHeaders{},
		},
		{
			"OK_1", "ich/du/sie",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SpecialHeaders:
					*v = SpecialHeaders{Decrypted: true, HeaderSize: 345, SegmentedObjectSize: 1098}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			SpecialHeaders{Decrypted: true, HeaderSize: 345, SegmentedObjectSize: 1098},
		},
		{
			"OK_2", "left/right/left",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SpecialHeaders:
					*v = SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: 90}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: 90},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			headers, err := GetSpecialHeaders(tt.path)

			if tt.expectedHeaders == (SpecialHeaders{}) {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedHeaders.Decrypted != headers.Decrypted {
				t.Errorf("Field 'Decrypted' incorrect. Expected %t, got %t", tt.expectedHeaders.Decrypted, headers.Decrypted)
			} else if tt.expectedHeaders.HeaderSize != headers.HeaderSize {
				t.Errorf("Field 'HeaderSize' incorrect. Expected %d, got %d", tt.expectedHeaders.HeaderSize, headers.HeaderSize)
			} else if tt.expectedHeaders.SegmentedObjectSize != headers.SegmentedObjectSize {
				t.Errorf("Field 'SegmentedObjectSize' incorrect. Expected %d, got %d",
					tt.expectedHeaders.SegmentedObjectSize, headers.SegmentedObjectSize)
			}
		})
	}
}*/
