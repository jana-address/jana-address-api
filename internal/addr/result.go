// Package addr defines the output contract of the batch endpoint.
package addr

import "encoding/json"

// Result is one parsed record. Key names and order follow the task spec exactly;
// unknown values are encoded as null (no omitempty).
type Result struct {
	ID         json.RawMessage `json:"id"`
	RawAddress json.RawMessage `json:"raw_address"`
	Province   *string         `json:"province"`
	City       *string         `json:"city"`
	District   *string         `json:"district"`
	LivingArea *string         `json:"living_area"`
	StreetType *string         `json:"street_type"`
	StreetName *string         `json:"street_name"`
	House      *string         `json:"house"`
	Apartment  *string         `json:"apartment"`
	Entrance   *string         `json:"entrance"`
	Floor      *string         `json:"floor"`
	Landmark   *string         `json:"landmark"`
	Confidence float64         `json:"confidence"`
}
