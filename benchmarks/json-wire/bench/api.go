package bench

import "context"

type Item struct {
	ID     string   `json:"id"`
	Count  int      `json:"count"`
	Active bool     `json:"active"`
	Score  float64  `json:"score"`
	Tags   []string `json:"tags"`
}

type EchoRequest struct {
	Name  string            `json:"name"`
	Count int               `json:"count"`
	Items []Item            `json:"items"`
	Meta  map[string]string `json:"meta"`
}

type EchoResponse struct {
	Name      string            `json:"name"`
	Count     int               `json:"count"`
	ItemCount int               `json:"item_count"`
	Total     int               `json:"total"`
	First     string            `json:"first"`
	Echo      []Item            `json:"echo"`
	Meta      map[string]string `json:"meta"`
}

//scenery:api public method=POST path=/echo
func Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	total := req.Count
	first := ""
	for i, item := range req.Items {
		total += item.Count
		if i == 0 {
			first = item.ID
		}
	}
	return &EchoResponse{
		Name:      req.Name,
		Count:     req.Count,
		ItemCount: len(req.Items),
		Total:     total,
		First:     first,
		Echo:      req.Items,
		Meta:      req.Meta,
	}, nil
}
