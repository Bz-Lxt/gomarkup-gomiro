package model

type Member struct {
	ID       string  `json:"id"`
	Nickname string  `json:"nickname"`
	Color    string  `json:"color"`
	UserIdx  uint32  `json:"userIdx"`
	CursorX  float64 `json:"cursorX,omitempty"`
	CursorY  float64 `json:"cursorY,omitempty"`
	Online   bool    `json:"online"`
}

type Selection struct {
	ClientID string   `json:"clientId"`
	ShapeIDs []string `json:"shapeIds"`
}
