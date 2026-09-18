package tolldata

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
)

// Station is a single toll gate/barrier from the OpenTollData reference dataset.
type Station struct {
	Name     string
	Operator string
	Type     string // "open" or "close"
	Lat      float64
	Lon      float64
}

// Dataset is the in-memory, ready-to-query form of the vendored OpenTollData snapshot.
type Dataset struct {
	Stations         []Station
	NetworkByStation map[string]string // station name -> network_name (closed-network connected component)
}

type rawTollDescription struct {
	OperatorRef string `json:"operator_ref"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Operator    string `json:"operator"`
	Type        string `json:"type"`
}

type rawNetwork struct {
	NetworkName string   `json:"network_name"`
	Tolls       []string `json:"tolls"`
}

type rawFile struct {
	TollDescription map[string]rawTollDescription `json:"toll_description"`
	Networks        []rawNetwork                  `json:"networks"`
}

// Load parses the embedded OpenTollData snapshot into an in-memory Dataset.
func Load() (*Dataset, error) {
	raw, err := FS.ReadFile("data/opentolldata_network_desc.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded toll dataset: %w", err)
	}

	var f rawFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("failed to parse embedded toll dataset: %w", err)
	}

	ds := &Dataset{
		Stations:         make([]Station, 0, len(f.TollDescription)),
		NetworkByStation: make(map[string]string, len(f.TollDescription)),
	}

	for name, td := range f.TollDescription {
		lat, errLat := strconv.ParseFloat(td.Lat, 64)
		lon, errLon := strconv.ParseFloat(td.Lon, 64)
		if errLat != nil || errLon != nil {
			log.Printf("[tolldata] skipping station %q: invalid coordinates (lat=%q lon=%q)", name, td.Lat, td.Lon)
			continue
		}
		ds.Stations = append(ds.Stations, Station{
			Name:     name,
			Operator: td.Operator,
			Type:     td.Type,
			Lat:      lat,
			Lon:      lon,
		})
	}

	for _, net := range f.Networks {
		for _, toll := range net.Tolls {
			ds.NetworkByStation[toll] = net.NetworkName
		}
	}

	return ds, nil
}
