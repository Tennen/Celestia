package market

type Estimate struct {
	EstimateNAV float64
	ChangePct   float64
	AsOf        string
}

type Security struct {
	Code string
	Name string
}
