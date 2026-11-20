package plugin

import "encoding/json"

// topClientRows flattens the top clients list. Meraki can't filter it by
// network, so networkID filters the organization's top 50 here.
func topClientRows(rows []json.RawMessage, networkID string) []json.RawMessage {
	type client struct {
		Name    string `json:"name"`
		Mac     string `json:"mac"`
		Network struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"network"`
		Usage struct {
			Total      float64 `json:"total"`
			Upstream   float64 `json:"upstream"`
			Downstream float64 `json:"downstream"`
			Percentage float64 `json:"percentage"`
		} `json:"usage"`
	}
	type row struct {
		Name        string  `json:"name"`
		Mac         string  `json:"mac"`
		NetworkName string  `json:"networkName"`
		Total       float64 `json:"total"`
		Downstream  float64 `json:"downstream"`
		Upstream    float64 `json:"upstream"`
		Percentage  float64 `json:"percentage"`
	}
	out := make([]row, 0, len(rows))
	for _, raw := range rows {
		var c client
		if json.Unmarshal(raw, &c) != nil || (networkID != "" && c.Network.ID != networkID) {
			continue
		}
		out = append(out, row{c.Name, c.Mac, c.Network.Name, c.Usage.Total, c.Usage.Downstream, c.Usage.Upstream, c.Usage.Percentage})
	}
	return marshalRows(out)
}
