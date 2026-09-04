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

type geoJSONFeatureCollection struct {
	Features []geoJSONFeature `json:"features"`
}

type geoJSONFeature struct {
	Properties geoJSONProperties `json:"properties"`
	Geometry   *geoJSONGeometry  `json:"geometry"`
}

type geoJSONProperties struct {
	SID          string `json:"sid"`
	Network      string `json:"network"`
	SName        string `json:"sname"`
	TzName       string `json:"tzname"`
	ArchiveBegin string `json:"archive_begin"`
}

type geoJSONGeometry struct {
	Coordinates []float64 `json:"coordinates"`
}

// Station is one IEM ASOS site from network GeoJSON.
type Station struct {
	SID          string
	Network      string
	Name         string
	TzName       string
	Latitude     *float64
	Longitude    *float64
	ArchiveBegin string
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

// LoadStations downloads US ASOS network GeoJSON and returns stations keyed by uppercase sid.
func (c *NetworkCatalog) LoadStations(ctx context.Context) (map[string]Station, error) {
	stations := make(map[string]Station)

	for _, network := range c.networks {
		networkStations, err := c.loadNetworkStations(ctx, network)
		if err != nil {
			return nil, err
		}

		for sid, station := range networkStations {
			if _, exists := stations[sid]; exists {
				continue
			}

			stations[sid] = station
		}
	}

	return stations, nil
}

// LoadStationIDs downloads US ASOS network GeoJSON and returns IEM site ids.
func (c *NetworkCatalog) LoadStationIDs(ctx context.Context) (map[string]struct{}, error) {
	stations, err := c.LoadStations(ctx)
	if err != nil {
		return nil, err
	}

	ids := make(map[string]struct{}, len(stations))
	for sid := range stations {
		ids[sid] = struct{}{}
	}

	return ids, nil
}

func (c *NetworkCatalog) loadNetworkStations(ctx context.Context, network string) (map[string]Station, error) {
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
		return map[string]Station{}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download geojson for %s: unexpected status %s", network, resp.Status)
	}

	var payload geoJSONFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode geojson for %s: %w", network, err)
	}

	stations := make(map[string]Station, len(payload.Features))
	for _, feature := range payload.Features {
		station, ok := stationFromFeature(feature, network)
		if !ok {
			continue
		}

		if _, exists := stations[station.SID]; exists {
			continue
		}

		stations[station.SID] = station
	}

	return stations, nil
}

func stationFromFeature(feature geoJSONFeature, fallbackNetwork string) (Station, bool) {
	sid := strings.ToUpper(strings.TrimSpace(feature.Properties.SID))
	if sid == "" {
		return Station{}, false
	}

	network := strings.TrimSpace(feature.Properties.Network)
	if network == "" {
		network = fallbackNetwork
	}

	lat, lon := coordsFromGeometry(feature.Geometry)

	return Station{
		SID:          sid,
		Network:      network,
		Name:         strings.TrimSpace(feature.Properties.SName),
		TzName:       strings.TrimSpace(feature.Properties.TzName),
		Latitude:     lat,
		Longitude:    lon,
		ArchiveBegin: strings.TrimSpace(feature.Properties.ArchiveBegin),
	}, true
}

func coordsFromGeometry(geometry *geoJSONGeometry) (lat, lon *float64) {
	if geometry == nil || len(geometry.Coordinates) < 2 {
		return nil, nil
	}

	lonV := geometry.Coordinates[0]
	latV := geometry.Coordinates[1]

	return &latV, &lonV
}
