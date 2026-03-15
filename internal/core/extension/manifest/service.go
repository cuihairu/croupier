package manifest

import "encoding/json"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Parse(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) MustJSON(raw string) map[string]any {
	parsed, err := s.Parse([]byte(raw))
	if err != nil {
		return map[string]any{}
	}
	return parsed
}
