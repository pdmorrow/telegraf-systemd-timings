package systemd_timings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// SystemdTimings is a telegraf plugin to gather systemd boot timing metrics.
type SystemdTimings struct {
	UnitPattern string `toml:"unitpattern"`
	Periodic    bool   `toml:"periodic"`
}

// Measurement name.
const measurement = "systemd_timings"

// Match on all service units by default.
const defaultUnitPattern = "*.service"

// Only run once by default.
const defaultPeriodic = false

// Record if we've collected everything (and thus do not need to collect again)
var collectionDone = false

// Map of a system wide boot metrics to their timestamps in microseconds, see:
// https://www.freedesktop.org/wiki/Software/systemd/dbus/ for more details.
var managerProps = map[string]string{
	"FirmwareTimestampMonotonic":               "",
	"LoaderTimestampMonotonic":                 "",
	"InitRDTimestampMonotonic":                 "",
	"UserspaceTimestampMonotonic":              "",
	"FinishTimestampMonotonic":                 "",
	"SecurityStartTimestampMonotonic":          "",
	"SecurityFinishTimestampMonotonic":         "",
	"GeneratorsStartTimestampMonotonic":        "",
	"GeneratorsFinishTimestampMonotonic":       "",
	"UnitsLoadStartTimestampMonotonic":         "",
	"UnitsLoadFinishTimestampMonotonic":        "",
	"InitRDSecurityStartTimestampMonotonic":    "",
	"InitRDSecurityFinishTimestampMonotonic":   "",
	"InitRDGeneratorsStartTimestampMonotonic":  "",
	"InitRDGeneratorsFinishTimestampMonotonic": "",
	"InitRDUnitsLoadStartTimestampMonotonic":   "",
	"InitRDUnitsLoadFinishTimestampMonotonic":  "",
}

// stripType removes the dbus type from the string str to return only the value.
// See https://www.alteeve.com/w/List_of_DBus_data_types for dbus type
// information.
func stripType(str string) string {
	return strings.Split(str, " ")[1]
}

// getManagerProp retrieves the property value with name propName.
func getManagerProp(dbusConn *dbus.Conn, propName string) (string, error) {
	prop, err := dbusConn.GetManagerProperty(propName)
	if err != nil {
		return "", err
	}

	return stripType(prop), nil
}

// bootIsFinished returns true if systemd has completed all unit initialization.
func bootIsFinished() bool {
	// Connect to the systemd dbus.
	dbusConn, err := dbus.NewSystemConnection()
	if err != nil {
		return false
	}

	defer dbusConn.Close()

	// Read the "FinishTimestampMonotonic" manager property, this will be
	// non-zero if the system has finished initialization.
	progressStr, err := getManagerProp(dbusConn, "FinishTimestampMonotonic")
	if err != nil {
		return false
	}

	// Convert to an int for comparison.
	progressVal, err := strconv.ParseInt(progressStr, 10, 32)
	if err != nil {
		return false
	}

	return progressVal != 0
}

// postAllManagerProps reads all systemd manager properties and sends them to
// telegraf.
func postAllManagerProps(dbusConn *dbus.Conn, acc telegraf.Accumulator) error {

	// Read all properties and send non zero values to telegraf.
	for name := range managerProps {
		propVal, err := getManagerProp(dbusConn, name)
		if err != nil {
			continue
		} else {
			// Save since we might need the value later when computing per unit
			// time deltas.
			managerProps[name] = propVal
			if propVal == "" || propVal == "0" {
				// Skip zero valued properties, these indicate unset properties
				// in systemd.
				continue
			}

			value, err := strconv.ParseUint(propVal, 10, 64)
			if err != nil {
				acc.AddError(err)
				continue
			}

			// Build field and tag maps.
			tags := map[string]string{"SystemTimestamp": name}

			fields := map[string]interface{}{"SystemTimestampValue": value}

			// Send to telegraf.
			acc.AddFields(measurement, fields, tags)
		}
	}

	return nil
}

// query dbus to access unit startup timing data, all time measurements here
// are measured in microseconds.
func getUnitTimingData(dbusConn *dbus.Conn,
	unitName string,
	userSpaceStart uint64) (uint64, uint64, uint64, uint64, uint64, error) {

	// Retrieve all timing properties for this unit.
	activatingProp, err := dbusConn.GetUnitProperty(unitName,
		"InactiveExitTimestampMonotonic")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	activatedProp, err := dbusConn.GetUnitProperty(unitName,
		"ActiveEnterTimestampMonotonic")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	deactivatingProp, err := dbusConn.GetUnitProperty(unitName,
		"ActiveExitTimestampMonotonic")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	deactivatedProp, err := dbusConn.GetUnitProperty(unitName,
		"InactiveEnterTimestampMonotonic")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	// Convert all to uint64 types and subtract the user space start time
	// stamp to give us relative startup times.
	activating, err := strconv.ParseUint(
		stripType(activatingProp.Value.String()), 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	activated, err := strconv.ParseUint(
		stripType(activatedProp.Value.String()), 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	deactivating, err := strconv.ParseUint(
		stripType(deactivatingProp.Value.String()), 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	deactivated, err := strconv.ParseUint(
		stripType(deactivatedProp.Value.String()), 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if activating > 0 {
		activating -= userSpaceStart
	}

	if activated > 0 {
		activated -= userSpaceStart
	}

	if deactivating > 0 {
		deactivating -= userSpaceStart
	}

	if deactivated > 0 {
		deactivated -= userSpaceStart
	}

	runtime := uint64(0)
	if activated >= activating {
		runtime = activated - activating
	} else if deactivated >= activating {
		runtime = deactivated - activating
	}

	// Return the timing data for this unit, converted to seconds.
	return activating, activated, deactivating, deactivated, runtime, nil
}

// postAllUnitTimingData
func postAllUnitTimingData(dbusConn *dbus.Conn,
	acc telegraf.Accumulator,
	s *SystemdTimings) error {
	statusList, err := dbusConn.ListUnitsByPatterns([]string{},
		strings.Split(s.UnitPattern, ","))
	if err != nil {
		acc.AddError(err)
		return err
	}

	// Get the user space start timestamp so we can subtract it from all
	// unit timestamps to give us a relative offset from user space start.
	userTs, found := managerProps["UserspaceTimestampMonotonic"]
	if !found {
		return fmt.Errorf(`UserspaceTimestampMonotonic not found, cannot
						  compute unit timestamps`)
	}

	// Convert UserspaceTimestampMonotonic to a uint64
	userStartTs, err := strconv.ParseUint(userTs, 10, 64)
	if err != nil {
		acc.AddError(err)
		return err
	}

	// For each unit query timing data, don't stop on failure.
	for _, unitStatus := range statusList {
		activating, activated, deactivating, deactivated, runtime, err :=
			getUnitTimingData(dbusConn, unitStatus.Name, userStartTs)
		if err != nil {
			acc.AddError(err)
		} else {
			if runtime == 0 && !strings.HasSuffix(unitStatus.Name, ".target") {
				// Don't post results for services which were never started
				// or stopped.
				continue
			}

			// These are per unit wide timestamps, so tag them as such.
			tags := map[string]string{"UnitName": unitStatus.Name}

			// Construct fields map.
			fields := map[string]interface{}{
				"ActivatingTimestamp":   activating,
				"ActivatedTimestamp":    activated,
				"DeactivatingTimestamp": deactivating,
				"DeactivatedTimestamp":  deactivated,
				"RunDuration":           runtime,
			}

			// Send to telegraf.
			acc.AddFields(measurement, fields, tags)
		}
	}

	return nil
}

// Description returns a short description of the plugin
func (s *SystemdTimings) Description() string {
	return "Gather systemd boot and unit timing data"
}

// SampleConfig returns sample configuration options.
func (s *SystemdTimings) SampleConfig() string {
	return `
  ## Filter for a specific unit name pattern, default is "*.service".  This
  # can be a comma separated list of patterns.
  # unitpattern = "*.service"
  ## By default this plugin collects metrics once, if you'd like to
  # continuously send (potentially) the same data periodically then set
  # this configuration option to true.
  # periodic = false
`
}

// Gather reads timestamp metrics from systemd via dbus and sends them to
// telegraf.
func (s *SystemdTimings) Gather(acc telegraf.Accumulator) error {
	if !bootIsFinished() {
		// We are not ready to collect yet, telegraf will call us later to
		// try again.
		return nil
	}

	if s.Periodic == false {
		// We only want to run once.
		if collectionDone == true {
			// By default we only collect once since these are generally boot
			// time metrics.
			return nil
		}
	}

	// Connect to the systemd dbus.
	dbusConn, err := dbus.NewSystemConnection()
	if err != nil {
		return err
	}

	defer dbusConn.Close()

	err = postAllManagerProps(dbusConn, acc)
	if err != nil {
		acc.AddError(err)
		return err
	}

	// Read all unit timing data.
	err = postAllUnitTimingData(dbusConn, acc, s)
	if err != nil {
		acc.AddError(err)
		return err
	}

	if err == nil {
		collectionDone = true
	}

	return err
}

func init() {
	inputs.Add("systemd_timings", func() telegraf.Input {
		return &SystemdTimings{
			UnitPattern: defaultUnitPattern,
			Periodic:    defaultPeriodic,
		}
	})
}
