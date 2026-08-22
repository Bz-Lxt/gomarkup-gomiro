package model

import "time"

type Board struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	HasPass   bool      `json:"hasPass"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	PassHash  string    `json:"-"`
}

type BoardListItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	HasPass   bool   `json:"hasPass"`
	Thumbnail string `json:"thumbnail,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Snapshot struct {
	BoardID   string            `json:"boardId"`
	ServerSeq uint64            `json:"serverSeq"`
	Shapes    map[string]*Shape `json:"shapes"`
	Groups    map[string][]string `json:"groups,omitempty"`
}
