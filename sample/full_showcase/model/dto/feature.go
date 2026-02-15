package dto

import "time"

// Feature is the feature DTO.
type Feature struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}
