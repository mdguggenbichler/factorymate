package frm

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleID unmarshals FRM entity IDs that may be JSON strings or numbers.
type FlexibleID string

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexibleID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexibleID(s)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}
	*f = FlexibleID(num.String())
	return nil
}

// String returns the ID as a plain string.
func (f FlexibleID) String() string {
	return string(f)
}

// FlexibleFloat unmarshals numeric fields that FRM may return as numbers or strings.
type FlexibleFloat float64

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = 0
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("parse float string %q: %w", s, err)
		}
		*f = FlexibleFloat(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*f = FlexibleFloat(v)
	return nil
}

// Float64 returns the value as float64.
func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}

// FlexibleAmount unmarshals item amounts that may be JSON strings or integers.
type FlexibleAmount struct {
	Value string
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *FlexibleAmount) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		a.Value = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &a.Value)
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}
	a.Value = num.String()
	return nil
}
