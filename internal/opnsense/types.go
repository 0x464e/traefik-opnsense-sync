package opnsense

type searchHostResponse struct {
	Rows []struct {
		UUID        string `json:"uuid"`
		Hostname    string `json:"hostname"`
		Domain      string `json:"domain"`
		Description string `json:"description"`
	} `json:"rows"`
}

type hostAliasCreate struct {
	Enabled     string `json:"enabled"`
	Host        string `json:"host"`
	Hostname    string `json:"hostname"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
}

type hostOverride struct {
	UUID     string
	Hostname string
	Domain   string
}
