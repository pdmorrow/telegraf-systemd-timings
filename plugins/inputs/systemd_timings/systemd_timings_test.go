package systemd_timings

import (
	"strings"
	"testing"

	"github.com/influxdata/telegraf/testutil"
)

func TestSystemdTiming(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		systemdTimings := &SystemdTimings{}
		acc := new(testutil.Accumulator)
		err := acc.GatherError(systemdTimings.Gather)
		if err != nil {
			t.Errorf("failed: %s\n", err)
		}

		for _, metric := range acc.Metrics {
			for tag, _ := range metric.Tags {
				if strings.Compare(tag, "SystemTimestamp") == 0 {
					for k, _ := range metric.Fields {
						if strings.Compare(k, "SystemTimestampValue") != 0 {
							t.Errorf("unexpected metric key \"%s\", "+
								"expected \"SystemTimestampValue\"\n", k)
						}
					}
				} else if strings.Compare(tag, "UnitName") == 0 {
					for k, _ := range metric.Fields {
						switch k {
						case "ActivatingTimestamp":
						// Do nothing.
						case "ActivatedTimestamp":
						// Do nothing.
						case "DeactivatingTimestamp":
						// Do nothing.
						case "DeactivatedTimestamp":
						// Do nothing.
						case "RunDuration":
						// Do nothing.
						default:
							t.Errorf("Unexpected key: %s\n", k)
						}
					}
				} else {
					t.Errorf("failed, unexpected tag: %s, expected any "+
						"of [SystemTimestamp, UnitName]\n", tag)
				}
			}
		}
	})
}
