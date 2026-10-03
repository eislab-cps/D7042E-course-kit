package frame

// KeySpec describes one registered key (section 3).
type KeySpec struct {
	Key   string
	Unit  string
	Scale int32 // physical value = raw / Scale
	Min   int32 // valid raw range, inclusive
	Max   int32
}

// Keys is the section 3 table. Custom scenarios add their keys here.
var Keys = map[string]KeySpec{
	"TEMP": {"TEMP", "°C", 100, -4000, 8500},
	"HUM":  {"HUM", "%RH", 100, 0, 10000},
	"BARO": {"BARO", "hPa", 10, 3000, 11000},
	"PWR":  {"PWR", "W", 10, 0, 1000000},
	"OCC":  {"OCC", "", 1, 0, 1},
	"CO2":  {"CO2", "ppm", 1, 0, 10000},
	"PRES": {"PRES", "bar", 100, 0, 40000},
	"CYC":  {"CYC", "cycles", 1, 0, 2147483647},
	"BOOT": {"BOOT", "", 1, 1, 65535},
}

// Checked is the result of applying the key table to a parsed frame.
type Checked struct {
	Valid   []Field  // known keys within range
	Unknown []string // ignored for forward compatibility
	Range   []Field  // known keys out of range; dropped
}

// Check applies section 3: unknown keys are ignored, out-of-range fields are dropped.
func Check(f Frame) Checked {
	var c Checked
	for _, fl := range f.Fields {
		spec, ok := Keys[fl.Key]
		switch {
		case !ok:
			c.Unknown = append(c.Unknown, fl.Key)
		case fl.Value < spec.Min || fl.Value > spec.Max:
			c.Range = append(c.Range, fl)
		default:
			c.Valid = append(c.Valid, fl)
		}
	}
	return c
}
