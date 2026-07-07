package bts

import (
	"strings"
	"unicode"
)

const (
	colYear       = "year"
	colDayOfMonth = "day_of_month"
	jsonKeyMonth  = "month"
	jsonKeyRows   = "rows_imported"
)

// DBColumns is the canonical on_time_flights column order (snake_case).
var DBColumns = []string{
	colYear,
	"quarter",
	"month",
	colDayOfMonth,
	"day_of_week",
	"flight_date",
	"marketing_airline_network",
	"operated_or_branded_code_share_partners",
	"dot_id_marketing_airline",
	"iata_code_marketing_airline",
	"flight_number_marketing_airline",
	"originally_scheduled_code_share_airline",
	"dot_id_originally_scheduled_code_share_airline",
	"iata_code_originally_scheduled_code_share_airline",
	"flight_num_originally_scheduled_code_share_airline",
	"operating_airline",
	"dot_id_operating_airline",
	"iata_code_operating_airline",
	"tail_number",
	"flight_number_operating_airline",
	"origin_airport_id",
	"origin_airport_seq_id",
	"origin_city_market_id",
	"origin",
	"origin_city_name",
	"origin_state",
	"origin_state_fips",
	"origin_state_name",
	"origin_wac",
	"dest_airport_id",
	"dest_airport_seq_id",
	"dest_city_market_id",
	"dest",
	"dest_city_name",
	"dest_state",
	"dest_state_fips",
	"dest_state_name",
	"dest_wac",
	"crs_dep_time",
	"dep_time",
	"dep_delay",
	"dep_delay_minutes",
	"dep_del15",
	"departure_delay_groups",
	"dep_time_blk",
	"taxi_out",
	"wheels_off",
	"wheels_on",
	"taxi_in",
	"crs_arr_time",
	"arr_time",
	"arr_delay",
	"arr_delay_minutes",
	"arr_del15",
	"arrival_delay_groups",
	"arr_time_blk",
	"cancelled",
	"cancellation_code",
	"diverted",
	"crs_elapsed_time",
	"actual_elapsed_time",
	"air_time",
	"flights",
	"distance",
	"distance_group",
	"carrier_delay",
	"weather_delay",
	"nas_delay",
	"security_delay",
	"late_aircraft_delay",
	"first_dep_time",
	"total_add_g_time",
	"longest_add_g_time",
	"div_airport_landings",
	"div_reached_dest",
	"div_actual_elapsed_time",
	"div_arr_delay",
	"div_distance",
	"div1_airport",
	"div1_airport_id",
	"div1_airport_seq_id",
	"div1_wheels_on",
	"div1_total_g_time",
	"div1_longest_g_time",
	"div1_wheels_off",
	"div1_tail_num",
	"div2_airport",
	"div2_airport_id",
	"div2_airport_seq_id",
	"div2_wheels_on",
	"div2_total_g_time",
	"div2_longest_g_time",
	"div2_wheels_off",
	"div2_tail_num",
	"div3_airport",
	"div3_airport_id",
	"div3_airport_seq_id",
	"div3_wheels_on",
	"div3_total_g_time",
	"div3_longest_g_time",
	"div3_wheels_off",
	"div3_tail_num",
	"div4_airport",
	"div4_airport_id",
	"div4_airport_seq_id",
	"div4_wheels_on",
	"div4_total_g_time",
	"div4_longest_g_time",
	"div4_wheels_off",
	"div4_tail_num",
	"div5_airport",
	"div5_airport_id",
	"div5_airport_seq_id",
	"div5_wheels_on",
	"div5_total_g_time",
	"div5_longest_g_time",
	"div5_wheels_off",
	"div5_tail_num",
	"duplicate",
}

var headerAliases = map[string]string{
	"DayofMonth": colDayOfMonth,
}

func csvHeaderToColumn(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	if col, ok := headerAliases[header]; ok {
		return col
	}

	return pascalToSnake(header)
}

func pascalToSnake(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s) + 8)

	for i, r := range s {
		if r == '_' {
			if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
				b.WriteRune('_')
			}

			continue
		}

		if i > 0 && unicode.IsUpper(r) {
			prev := rune(s[i-1])
			if prev != '_' && (!unicode.IsUpper(prev) || (i+1 < len(s) && unicode.IsLower(rune(s[i+1])))) {
				b.WriteRune('_')
			}
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}
