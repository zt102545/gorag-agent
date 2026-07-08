package qa

import "time"

type Item struct {
	ID        string    `json:"id"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Category  string    `json:"category,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateRequest struct {
	Question string   `json:"question" binding:"required"`
	Answer   string   `json:"answer" binding:"required"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type UpdateRequest struct {
	Question string   `json:"question" binding:"required"`
	Answer   string   `json:"answer" binding:"required"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type ListQuery struct {
	Category string
	Page     int
	Size     int
}

type ListResult struct {
	Total int64  `json:"total"`
	Items []Item `json:"items"`
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}
