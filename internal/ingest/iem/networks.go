package iem

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultGeoJSONBaseURL = "https://mesonet.agron.iastate.edu/geojson/network"

// usASOSNetworks is the IEM ASOS network list for US states and territories
// (same set used by IEM's official scraper example).
var usASOSNetworks = []string{
	"AK_ASOS", "AL_ASOS", "AR_ASOS", "AZ_ASOS", "CA_ASOS", "CO_ASOS", "CT_ASOS",
	"DE_ASOS", "FL_ASOS", "GA_ASOS", "HI_ASOS", "IA_ASOS", "ID_ASOS", "IL_ASOS",
	"IN_ASOS", "KS_ASOS", "KY_ASOS", "LA_ASOS", "MA_ASOS", "MD_ASOS", "ME_ASOS",
	"MI_ASOS", "MN_ASOS", "MO_ASOS", "MS_ASOS", "MT_ASOS", "NC_ASOS", "ND_ASOS",
	"NE_ASOS", "NH_ASOS", "NJ_ASOS", "NM_ASOS", "NV_ASOS", "NY_ASOS", "OH_ASOS",
	"OK_ASOS", "OR_ASOS", "PA_ASOS", "RI_ASOS", "SC_ASOS", "SD_ASOS", "TN_ASOS",
	"TX_ASOS", "UT_ASOS", "VA_ASOS", "VT_ASOS", "WA_ASOS", "WI_ASOS", "WV_ASOS",
	"WY_ASOS", "PR_ASOS", "VI_ASOS", "GU_ASOS", "AS_ASOS", "MP_ASOS",
}

type NetworkCatalog struct {
	baseURL    string
	httpClient *http.Client
	networks   []string
}

func NewNetworkCatalog(baseURL string, timeout time.Duration) *NetworkCatalog {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultGeoJSONBaseURL
	}

	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	return &NetworkCatalog{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		networks: usASOSNetworks,
	}
}

type geoJSONFeatureCollection struct {
	Features []struct {
		Properties struct {
			SID string `json:"sid"`
		} `json:"properties"`
	} `json:"features"`
}

// LoadStationIDs downloads US ASOS network GeoJSON and returns IEM site ids.
func (c *NetworkCatalog) LoadStationIDs(ctx context.Context) (map[string]struct{}, error) {
	ids := make(map[string]struct{})

	for _, network := range c.networks {
		networkIDs, err := c.loadNetwork(ctx, network)
		if err != nil {
			return nil, err
		}

		for id := range networkIDs {
			ids[id] = struct{}{}
		}
	}

	return ids, nil
}

func (c *NetworkCatalog) loadNetwork(ctx context.Context, network string) (map[string]struct{}, error) {
	reqURL := fmt.Sprintf("%s/%s.geojson", c.baseURL, network)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build geojson request for %s: %w", network, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download geojson for %s: %w", network, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Some territory networks may be absent; skip quietly.
		return map[string]struct{}{}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download geojson for %s: unexpected status %s", network, resp.Status)
	}

	var payload geoJSONFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode geojson for %s: %w", network, err)
	}

	ids := make(map[string]struct{}, len(payload.Features))
	for _, feature := range payload.Features {
		sid := strings.ToUpper(strings.TrimSpace(feature.Properties.SID))
		if sid == "" {
			continue
		}

		ids[sid] = struct{}{}
	}

	return ids, nil
}
