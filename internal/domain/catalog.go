package domain

type EntityCatalog struct {
	Projects []Project `json:"projects"`
	Areas    []Area    `json:"areas"`
	People   []Person  `json:"people"`
	Tags     []string  `json:"tags"`
	Contexts []string  `json:"contexts"`
}
