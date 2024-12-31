package domain

type FileContentResponse struct {
	Filename  string `json:"filename"`
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Position  int    `json:"position"`
}
