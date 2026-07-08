package model

type OnTimeFlight struct {
	FlightDate                      string `json:"flight_date"`
	DayOfWeek                       string `json:"day_of_week,omitempty"`
	Origin                          string `json:"origin"`
	Dest                            string `json:"dest"`
	IATA_Code_Marketing_Airline     string `json:"iata_code_marketing_airline"`
	Flight_Number_Marketing_Airline string `json:"flight_number_marketing_airline"`
	IATA_Code_Operating_Airline     string `json:"iata_code_operating_airline"`
	Flight_Number_Operating_Airline string `json:"flight_number_operating_airline"`
	CRSDepTime                      string `json:"crs_dep_time"`
	DepTime                         string `json:"dep_time"`
	DepDelay                        string `json:"dep_delay"`
	DepDelayMinutes                 string `json:"dep_delay_minutes,omitempty"`
	DepDel15                        string `json:"dep_del15,omitempty"`
	CRSArrTime                      string `json:"crs_arr_time"`
	ArrTime                         string `json:"arr_time"`
	ArrDelay                        string `json:"arr_delay"`
	ArrDelayMinutes                 string `json:"arr_delay_minutes,omitempty"`
	ArrDel15                        string `json:"arr_del15,omitempty"`
	Cancelled                       string `json:"cancelled"`
	CancellationCode                string `json:"cancellation_code,omitempty"`
	Diverted                        string `json:"diverted"`
	Distance                        string `json:"distance"`
	CarrierDelay                    string `json:"carrier_delay,omitempty"`
	WeatherDelay                    string `json:"weather_delay,omitempty"`
	NASDelay                        string `json:"nas_delay,omitempty"`
	SecurityDelay                   string `json:"security_delay,omitempty"`
	LateAircraftDelay               string `json:"late_aircraft_delay,omitempty"`
	Div1Airport                     string `json:"div1_airport,omitempty"`
	Div2Airport                     string `json:"div2_airport,omitempty"`
	Div3Airport                     string `json:"div3_airport,omitempty"`
	Div4Airport                     string `json:"div4_airport,omitempty"`
	Div5Airport                     string `json:"div5_airport,omitempty"`
}
