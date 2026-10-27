package domain

import "fmt"

// Workstation is an IRBIS ARM code (single letter).
type Workstation string

const (
	ARMAdministrator Workstation = "A"
	ARMCataloger     Workstation = "C"
	ARMAcquisitions  Workstation = "M"
	ARMReader        Workstation = "R"
	ARMCirculation   Workstation = "B"
	ARMProvision     Workstation = "K"
)

// WorkstationInfo is a known ARM for clients.
type WorkstationInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// KnownWorkstations is the fixed IRBIS ARM list exposed to clients.
var KnownWorkstations = []WorkstationInfo{
	{Code: "A", Name: "Administrator"},
	{Code: "C", Name: "Cataloger"},
	{Code: "M", Name: "Acquisitions"},
	{Code: "R", Name: "Reader"},
	{Code: "B", Name: "Circulation"},
	{Code: "K", Name: "Provision"},
}

// NormalizeWorkstation validates and uppercases an ARM letter.
// Empty input returns defaultARM (typically Cataloger).
func NormalizeWorkstation(raw string, defaultARM Workstation) (Workstation, error) {
	if raw == "" {
		if defaultARM == "" {
			defaultARM = ARMCataloger
		}
		return defaultARM, nil
	}
	if len(raw) != 1 {
		return "", fmt.Errorf("%w: workstation must be one letter", ErrInvalidInput)
	}
	c := raw[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	switch Workstation(c) {
	case ARMAdministrator, ARMCataloger, ARMAcquisitions, ARMReader, ARMCirculation, ARMProvision:
		return Workstation(c), nil
	default:
		return "", fmt.Errorf("%w: unknown workstation %q", ErrInvalidInput, raw)
	}
}
