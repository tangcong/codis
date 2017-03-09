package models

type ClusterConfig struct {
	DashboardAddr string `json:"dashboard_addr"`
	ProductName   string `json:"product_name"`
	ProductAuth   string `json:"product_auth"`
}

func (p *ClusterConfig) Encode() []byte {
	return jsonEncode(p)
}
