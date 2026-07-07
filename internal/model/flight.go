package model

type OnTimeFlight struct {
	FlightDate                      string `json:"flight_date"`
	Origin                          string `json:"origin"`
	Dest                            string `json:"dest"`
	IATA_Code_Marketing_Airline     string `json:"iata_code_marketing_airline"`
	Flight_Number_Marketing_Airline string `json:"flight_number_marketing_airline"`
	IATA_Code_Operating_Airline     string `json:"iata_code_operating_airline"`
	Flight_Number_Operating_Airline string `json:"flight_number_operating_airline"`
	CRSDepTime                      string `json:"crs_dep_time"`
	DepTime                         string `json:"dep_time"`
	DepDelay                        string `json:"dep_delay"`
	CRSArrTime                      string `json:"crs_arr_time"`
	ArrTime                         string `json:"arr_time"`
	ArrDelay                        string `json:"arr_delay"`
	Cancelled                       string `json:"cancelled"`
	Diverted                        string `json:"diverted"`
	Distance                        string `json:"distance"`
}
