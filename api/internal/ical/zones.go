package ical

import (
	"fmt"
	"strings"
	"time"
)

// windowsZones maps Windows zone names (Outlook and Exchange TZID values)
// to IANA names. The source is the CLDR windowsZones table, territory
// "001". Keys are lower case.
var windowsZones = map[string]string{
	"dateline standard time":          "Etc/GMT+12",
	"utc-11":                          "Etc/GMT+11",
	"hawaiian standard time":          "Pacific/Honolulu",
	"alaskan standard time":           "America/Anchorage",
	"pacific standard time (mexico)":  "America/Tijuana",
	"pacific standard time":           "America/Los_Angeles",
	"us mountain standard time":       "America/Phoenix",
	"mountain standard time (mexico)": "America/Mazatlan",
	"mountain standard time":          "America/Denver",
	"central america standard time":   "America/Guatemala",
	"central standard time":           "America/Chicago",
	"central standard time (mexico)":  "America/Mexico_City",
	"canada central standard time":    "America/Regina",
	"sa pacific standard time":        "America/Bogota",
	"eastern standard time":           "America/New_York",
	"eastern standard time (mexico)":  "America/Cancun",
	"us eastern standard time":        "America/Indiana/Indianapolis",
	"venezuela standard time":         "America/Caracas",
	"atlantic standard time":          "America/Halifax",
	"sa western standard time":        "America/La_Paz",
	"central brazilian standard time": "America/Cuiaba",
	"pacific sa standard time":        "America/Santiago",
	"newfoundland standard time":      "America/St_Johns",
	"e. south america standard time":  "America/Sao_Paulo",
	"argentina standard time":         "America/Argentina/Buenos_Aires",
	"sa eastern standard time":        "America/Cayenne",
	"greenland standard time":         "America/Nuuk",
	"montevideo standard time":        "America/Montevideo",
	"utc-02":                          "Etc/GMT+2",
	"azores standard time":            "Atlantic/Azores",
	"cape verde standard time":        "Atlantic/Cape_Verde",
	"utc":                             "UTC",
	"coordinated universal time":      "UTC",
	"gmt standard time":               "Europe/London",
	"greenwich standard time":         "Atlantic/Reykjavik",
	"morocco standard time":           "Africa/Casablanca",
	"w. europe standard time":         "Europe/Berlin",
	"central europe standard time":    "Europe/Budapest",
	"romance standard time":           "Europe/Paris",
	"central european standard time":  "Europe/Warsaw",
	"w. central africa standard time": "Africa/Lagos",
	"gtb standard time":               "Europe/Bucharest",
	"e. europe standard time":         "Europe/Chisinau",
	"egypt standard time":             "Africa/Cairo",
	"fle standard time":               "Europe/Kiev",
	"israel standard time":            "Asia/Jerusalem",
	"south africa standard time":      "Africa/Johannesburg",
	"russian standard time":           "Europe/Moscow",
	"arab standard time":              "Asia/Riyadh",
	"e. africa standard time":         "Africa/Nairobi",
	"arabic standard time":            "Asia/Baghdad",
	"turkey standard time":            "Europe/Istanbul",
	"iran standard time":              "Asia/Tehran",
	"arabian standard time":           "Asia/Dubai",
	"afghanistan standard time":       "Asia/Kabul",
	"pakistan standard time":          "Asia/Karachi",
	"west asia standard time":         "Asia/Tashkent",
	"ekaterinburg standard time":      "Asia/Yekaterinburg",
	"india standard time":             "Asia/Kolkata",
	"sri lanka standard time":         "Asia/Colombo",
	"nepal standard time":             "Asia/Kathmandu",
	"central asia standard time":      "Asia/Almaty",
	"bangladesh standard time":        "Asia/Dhaka",
	"myanmar standard time":           "Asia/Yangon",
	"se asia standard time":           "Asia/Bangkok",
	"north asia standard time":        "Asia/Krasnoyarsk",
	"china standard time":             "Asia/Shanghai",
	"singapore standard time":         "Asia/Singapore",
	"w. australia standard time":      "Australia/Perth",
	"taipei standard time":            "Asia/Taipei",
	"tokyo standard time":             "Asia/Tokyo",
	"korea standard time":             "Asia/Seoul",
	"cen. australia standard time":    "Australia/Adelaide",
	"aus central standard time":       "Australia/Darwin",
	"e. australia standard time":      "Australia/Brisbane",
	"aus eastern standard time":       "Australia/Sydney",
	"west pacific standard time":      "Pacific/Port_Moresby",
	"tasmania standard time":          "Australia/Hobart",
	"vladivostok standard time":       "Asia/Vladivostok",
	"new zealand standard time":       "Pacific/Auckland",
	"fiji standard time":              "Pacific/Fiji",
	"tonga standard time":             "Pacific/Tongatapu",
	"samoa standard time":             "Pacific/Apia",
	"line islands standard time":      "Pacific/Kiritimati",
}

// windowsDisplayZones maps Outlook display names (the text after the
// "(UTC-07:00) " prefix) to IANA names. Keys are lower case.
var windowsDisplayZones = map[string]string{
	"mountain time (us & canada)":       "America/Denver",
	"pacific time (us & canada)":        "America/Los_Angeles",
	"central time (us & canada)":        "America/Chicago",
	"eastern time (us & canada)":        "America/New_York",
	"arizona":                           "America/Phoenix",
	"alaska":                            "America/Anchorage",
	"hawaii":                            "Pacific/Honolulu",
	"atlantic time (canada)":            "America/Halifax",
	"dublin, edinburgh, lisbon, london": "Europe/London",
	"amsterdam, berlin, bern, rome, stockholm, vienna":  "Europe/Berlin",
	"brussels, copenhagen, madrid, paris":               "Europe/Paris",
	"belgrade, bratislava, budapest, ljubljana, prague": "Europe/Budapest",
	"sarajevo, skopje, warsaw, zagreb":                  "Europe/Warsaw",
	"athens, bucharest":                                 "Europe/Bucharest",
	"helsinki, kyiv, riga, sofia, tallinn, vilnius":     "Europe/Kiev",
	"moscow, st. petersburg":                            "Europe/Moscow",
	"chennai, kolkata, mumbai, new delhi":               "Asia/Kolkata",
	"beijing, chongqing, hong kong, urumqi":             "Asia/Shanghai",
	"osaka, sapporo, tokyo":                             "Asia/Tokyo",
	"canberra, melbourne, sydney":                       "Australia/Sydney",
	"auckland, wellington":                              "Pacific/Auckland",
	"coordinated universal time":                        "UTC",
	"brasilia":                                          "America/Sao_Paulo",
	"bogota, lima, quito, rio branco":                   "America/Bogota",
	"guadalajara, mexico city, monterrey":               "America/Mexico_City",
	"kuala lumpur, singapore":                           "Asia/Singapore",
	"seoul":                                             "Asia/Seoul",
	"jerusalem":                                         "Asia/Jerusalem",
	"cairo":                                             "Africa/Cairo",
	"harare, pretoria":                                  "Africa/Johannesburg",
	"abu dhabi, muscat":                                 "Asia/Dubai",
	"islamabad, karachi":                                "Asia/Karachi",
	"bangkok, hanoi, jakarta":                           "Asia/Bangkok",
	"perth":                                             "Australia/Perth",
	"brisbane":                                          "Australia/Brisbane",
	"adelaide":                                          "Australia/Adelaide",
	"darwin":                                            "Australia/Darwin",
	"hobart":                                            "Australia/Hobart",
	"newfoundland":                                      "America/St_Johns",
	"saskatchewan":                                      "America/Regina",
	"indiana (east)":                                    "America/Indiana/Indianapolis",
	"istanbul":                                          "Europe/Istanbul",
	"tehran":                                            "Asia/Tehran",
	"kathmandu":                                         "Asia/Kathmandu",
	"taipei":                                            "Asia/Taipei",
	"monrovia, reykjavik":                               "Atlantic/Reykjavik",
	"west central africa":                               "Africa/Lagos",
	"nairobi":                                           "Africa/Nairobi",
	"baghdad":                                           "Asia/Baghdad",
	"kuwait, riyadh":                                    "Asia/Riyadh",
	"buenos aires":                                      "America/Argentina/Buenos_Aires",
	"santiago":                                          "America/Santiago",
	"caracas":                                           "America/Caracas",
	"georgetown, la paz, manaus, san juan":              "America/La_Paz",
	"central america":                                   "America/Guatemala",
	"chihuahua, la paz, mazatlan":                       "America/Mazatlan",
	"baja california":                                   "America/Tijuana",
	"yekaterinburg":                                     "Asia/Yekaterinburg",
	"astana":                                            "Asia/Almaty",
	"dhaka":                                             "Asia/Dhaka",
	"yangon (rangoon)":                                  "Asia/Yangon",
	"krasnoyarsk":                                       "Asia/Krasnoyarsk",
	"vladivostok":                                       "Asia/Vladivostok",
	"fiji":                                              "Pacific/Fiji",
	"nuku'alofa":                                        "Pacific/Tongatapu",
	"samoa":                                             "Pacific/Apia",
	"azores":                                            "Atlantic/Azores",
	"cabo verde is.":                                    "Atlantic/Cape_Verde",
	"casablanca":                                        "Africa/Casablanca",
	"sri jayawardenepura":                               "Asia/Colombo",
	"kabul":                                             "Asia/Kabul",
	"tashkent, ashgabat":                                "Asia/Tashkent",
	"guam, port moresby":                                "Pacific/Port_Moresby",
	"international date line west":                      "Etc/GMT+12",
	"coordinated universal time-11":                     "Etc/GMT+11",
	"cayenne, fortaleza":                                "America/Cayenne",
	"greenland":                                         "America/Nuuk",
	"montevideo":                                        "America/Montevideo",
	"cuiaba":                                            "America/Cuiaba",
	"chisinau":                                          "Europe/Chisinau",
	"kiritimati island":                                 "Pacific/Kiritimati",
	"chetumal":                                          "America/Cancun",
	"bogota, lima, quito":                               "America/Bogota",
	"edinburgh, london":                                 "Europe/London",
	"helsinki, kiev, riga, sofia, tallinn, vilnius":           "Europe/Kiev",
	"pacific time (us & canada); tijuana":                     "America/Los_Angeles",
	"eastern time (us & canada); indiana (east)":              "America/New_York",
	"central time (us & canada); saskatchewan":                "America/Chicago",
	"mountain time (us & canada); chihuahua, la paz":          "America/Denver",
	"greenwich mean time : dublin, edinburgh, lisbon, london": "Europe/London",
}

// resolveZoneName returns the IANA name for a TZID, or "" when it does
// not know the TZID. It tries, in order: the IANA database; the Windows
// zone names; the Outlook display names; the last two or three path
// parts of a TZID with a prefix (for example
// "/mozilla.org/20050126_1/America/New_York").
func resolveZoneName(tzid string) string {
	s := strings.Trim(strings.TrimSpace(tzid), `"`)
	if s == "" || strings.EqualFold(s, "local") {
		return ""
	}
	switch strings.ToUpper(s) {
	case "Z", "UTC", "GMT", "ETC/UTC", "ETC/GMT":
		return "UTC"
	}
	if _, err := time.LoadLocation(s); err == nil {
		return s
	}
	low := strings.ToLower(s)
	if n, ok := windowsZones[low]; ok {
		return n
	}
	disp := low
	if strings.HasPrefix(disp, "(utc") || strings.HasPrefix(disp, "(gmt") {
		if i := strings.Index(disp, ")"); i > 0 {
			disp = strings.TrimSpace(disp[i+1:])
		}
	}
	if n, ok := windowsDisplayZones[disp]; ok {
		return n
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	for _, k := range []int{3, 2} {
		if len(parts) > k {
			cand := strings.Join(parts[len(parts)-k:], "/")
			if _, err := time.LoadLocation(cand); err == nil {
				return cand
			}
		}
	}
	return ""
}

// zoneTransition is one change of UTC offset.
type zoneTransition struct {
	at       time.Time
	from, to int
	name     string
	dst      bool
}

// localOnset is the wall time of a transition in the offset before it.
// RFC 5545 uses this form for DTSTART in STANDARD and DAYLIGHT.
func (z zoneTransition) localOnset() time.Time {
	return z.at.UTC().Add(time.Duration(z.from) * time.Second)
}

// patterns returns the BYDAY ordinals that fit the local day of the
// transition: 1 to 4 for "nth weekday", and -1 for "last weekday".
func (z zoneTransition) patterns() (ords []int) {
	l := z.localOnset()
	day := l.Day()
	n := (day-1)/7 + 1
	daysIn := time.Date(l.Year(), l.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if n <= 4 {
		ords = append(ords, n)
	}
	if day+7 > daysIn {
		ords = append(ords, -1)
	}
	return ords
}

var icalWeekdays = [...]string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

// zoneTransitions lists the offset changes of loc from the start of
// fromYear to the end of toYear. It uses time.Time.ZoneBounds, so it
// works with any zone the Go tz database has.
func zoneTransitions(loc *time.Location, fromYear, toYear int) []zoneTransition {
	t := time.Date(fromYear, 1, 1, 0, 0, 0, 0, loc)
	end := time.Date(toYear+1, 1, 1, 0, 0, 0, 0, loc)
	var out []zoneTransition
	for i := 0; i < 4000; i++ {
		_, e := t.ZoneBounds()
		if e.IsZero() || !e.Before(end) {
			break
		}
		_, from := t.Zone()
		name, to := e.Zone()
		if from != to {
			out = append(out, zoneTransition{at: e, from: from, to: to, name: name, dst: e.IsDST()})
		}
		t = e
	}
	return out
}

// writeVTimezone writes a VTIMEZONE for an IANA zone, made from the Go tz
// database.
//
// Approach: list the real offset changes in [fromYear, toYear]. Split
// them into STANDARD and DAYLIGHT. Group each kind into runs with the
// same offsets, name, month, weekday rule, and local time, one per year.
// A run becomes one sub-component with a yearly RRULE (with UNTIL, or
// open when the run reaches toYear, so later dates still resolve). A
// change that fits no rule is a sub-component with no RRULE. A zone with
// no change in the span gets one STANDARD with its current offset.
func writeVTimezone(w *lineWriter, name string, loc *time.Location, fromYear, toYear int) {
	w.line("BEGIN", nil, "VTIMEZONE")
	w.line("TZID", nil, name)
	ts := zoneTransitions(loc, fromYear, toYear)
	if len(ts) == 0 {
		abbr, off := time.Date(toYear, 1, 1, 0, 0, 0, 0, loc).Zone()
		w.line("BEGIN", nil, "STANDARD")
		w.line("DTSTART", nil, "19700101T000000")
		w.line("TZOFFSETFROM", nil, formatOffset(off))
		w.line("TZOFFSETTO", nil, formatOffset(off))
		w.line("TZNAME", nil, escapeText(abbr))
		w.line("END", nil, "STANDARD")
		w.line("END", nil, "VTIMEZONE")
		return
	}
	for _, dst := range []bool{false, true} {
		var kind []zoneTransition
		for _, z := range ts {
			if z.dst == dst {
				kind = append(kind, z)
			}
		}
		runs := splitRuns(kind)
		for i, run := range runs {
			open := i == len(runs)-1 && run.trans[len(run.trans)-1].at.Year() >= toYear-1
			writeObservance(w, run, open)
		}
	}
	w.line("END", nil, "VTIMEZONE")
}

type zoneRun struct {
	trans []zoneTransition
	ords  []int
}

func intersect(a, b []int) []int {
	var out []int
	for _, x := range a {
		for _, y := range b {
			if x == y {
				out = append(out, x)
			}
		}
	}
	return out
}

// splitRuns groups transitions of one kind into runs that one yearly
// RRULE can describe.
func splitRuns(ts []zoneTransition) []zoneRun {
	var runs []zoneRun
	for _, z := range ts {
		if n := len(runs); n > 0 {
			r := &runs[n-1]
			p := r.trans[len(r.trans)-1]
			pl, zl := p.localOnset(), z.localOnset()
			ph, pm, ps := pl.Clock()
			zh, zm, zs := zl.Clock()
			if p.from == z.from && p.to == z.to && p.name == z.name &&
				pl.Month() == zl.Month() && pl.Weekday() == zl.Weekday() &&
				ph == zh && pm == zm && ps == zs && zl.Year() == pl.Year()+1 {
				if common := intersect(r.ords, z.patterns()); len(common) > 0 {
					r.trans = append(r.trans, z)
					r.ords = common
					continue
				}
			}
		}
		runs = append(runs, zoneRun{trans: []zoneTransition{z}, ords: z.patterns()})
	}
	return runs
}

func writeObservance(w *lineWriter, run zoneRun, open bool) {
	first := run.trans[0]
	last := run.trans[len(run.trans)-1]
	comp := "STANDARD"
	if first.dst {
		comp = "DAYLIGHT"
	}
	w.line("BEGIN", nil, comp)
	w.line("DTSTART", nil, first.localOnset().Format("20060102T150405"))
	w.line("TZOFFSETFROM", nil, formatOffset(first.from))
	w.line("TZOFFSETTO", nil, formatOffset(first.to))
	w.line("TZNAME", nil, escapeText(first.name))
	if len(run.ords) > 0 && (open || len(run.trans) > 1) {
		ord := run.ords[0]
		for _, o := range run.ords {
			if o == -1 {
				ord = -1
			}
		}
		l := first.localOnset()
		rule := fmt.Sprintf("FREQ=YEARLY;BYMONTH=%d;BYDAY=%d%s", int(l.Month()), ord, icalWeekdays[l.Weekday()])
		if !open {
			rule += ";UNTIL=" + last.at.UTC().Format("20060102T150405Z")
		}
		w.line("RRULE", nil, rule)
	} else if len(run.trans) > 1 {
		var ds []string
		for _, z := range run.trans[1:] {
			ds = append(ds, z.localOnset().Format("20060102T150405"))
		}
		w.line("RDATE", nil, strings.Join(ds, ","))
	}
	w.line("END", nil, comp)
}

// formatOffset writes a UTC offset as +HHMM, or +HHMMSS when it has
// seconds.
func formatOffset(sec int) string {
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	h, m, s := sec/3600, sec%3600/60, sec%60
	if s != 0 {
		return fmt.Sprintf("%s%02d%02d%02d", sign, h, m, s)
	}
	return fmt.Sprintf("%s%02d%02d", sign, h, m)
}
