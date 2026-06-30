package httpapi

// uploadLinkRequestDTO is the request body for POST /photos/{photoId}/upload-link.
type uploadLinkRequestDTO struct {
	ContentType string `json:"contentType"`
}

// uploadLinkResponseDTO is the response body for POST /photos/{photoId}/upload-link.
type uploadLinkResponseDTO struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt string            `json:"expiresAt"`
}

type bboxDTO struct {
	XMin float64 `json:"xMin"`
	YMin float64 `json:"yMin"`
	XMax float64 `json:"xMax"`
	YMax float64 `json:"yMax"`
}

type predictionDTO struct {
	ClassID    string  `json:"classId"`
	Confidence float64 `json:"confidence"`
	BBox       bboxDTO `json:"bbox"`
}

type photoDTO struct {
	ID          string          `json:"id"`
	X           float64         `json:"x"`
	Y           float64         `json:"y"`
	H           float64         `json:"h"`
	Width       int             `json:"width"`
	Height      int             `json:"height"`
	CapturedAt  string          `json:"capturedAt"`
	OriginalURL string          `json:"originalUrl"`
	Predictions []predictionDTO `json:"predictions"`
}

type photoPageResponseDTO struct {
	Items     []photoDTO `json:"items"`
	NextToken string     `json:"next_token,omitempty"`
}
