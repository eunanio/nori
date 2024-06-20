package config

type Config struct {
	Runtime string  `json:"runtime"`
	Remote  *string `json:"remote,omitempty"`
	Region  *string `json:"region,omitempty"`
}