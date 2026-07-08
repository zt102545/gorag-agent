package qa

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

func ParseCSV(r io.Reader) ([]CreateRequest, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("csv must have header and at least one data row")
	}

	header := records[0]
	colIndex := map[string]int{}
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"question", "answer"}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("csv missing required column: %s", col)
		}
	}

	var items []CreateRequest
	for rowIdx, record := range records[1:] {
		if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
			continue
		}
		item := CreateRequest{
			Question: strings.TrimSpace(record[colIndex["question"]]),
			Answer:   strings.TrimSpace(record[colIndex["answer"]]),
		}
		if item.Question == "" || item.Answer == "" {
			return nil, fmt.Errorf("row %d: question and answer are required", rowIdx+2)
		}
		if idx, ok := colIndex["category"]; ok && idx < len(record) {
			item.Category = strings.TrimSpace(record[idx])
		}
		if idx, ok := colIndex["tags"]; ok && idx < len(record) {
			item.Tags = parseTags(record[idx])
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no valid qa rows found in csv")
	}
	return items, nil
}

func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
