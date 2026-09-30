package contract

import "encoding/json"

// UnmarshalJSON gives provenance confidence an explicit wire default of 1
// without conflating an omitted value with a caller deliberately sending 0.
func (p *ProvenanceRecord) UnmarshalJSON(data []byte) error {
	type alias ProvenanceRecord
	var wire struct {
		*alias
		Confidence *float64 `json:"confidence"`
	}
	wire.alias = (*alias)(p)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Confidence == nil {
		p.Confidence = 1
	} else {
		p.Confidence = *wire.Confidence
	}
	return nil
}
