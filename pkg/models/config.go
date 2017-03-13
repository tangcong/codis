package models

type ClusterConfig struct {
	DashboardAddr string `json:"dashboard_addr"`
	ProductName   string `json:"product_name"`
	ProductAuth   string `json:"product_auth"`
	BackupAddr    string `json:"backup_addr"`
}

func (p *ClusterConfig) Encode() []byte {
	return jsonEncode(p)
}
