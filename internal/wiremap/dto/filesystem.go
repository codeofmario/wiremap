package dto

type FileEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"isDir"`
	Size        int64  `json:"size"`
	Permissions string `json:"permissions"`
}

type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Binary  bool   `json:"binary"`
	Size    int64  `json:"size"`
}
