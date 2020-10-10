# systemd_timings Input Plugin

The systemd_timings plugin collects many timestamps relating to the systemd
based boot process.  All values are accessed via systemd APIs which are exposed
via dbus. For more information on the systemd dbus API see:

   * https://www.freedesktop.org/wiki/Software/systemd/dbus/

## System Wide Boot Timestamps

The values produced here indicate timestamps of various system wide boot tasks:

   * FirmwareTimestampMonotonic
   * LoaderTimestampMonotonic
   * InitRDTimestampMonotonic
   * UserspaceTimestampMonotonic
   * FinishTimestampMonotonic
   * SecurityStartTimestampMonotonic
   * SecurityFinishTimestampMonotonic
   * GeneratorsStartTimestampMonotonic
   * GeneratorsFinishTimestampMonotonic
   * UnitsLoadStartTimestampMonotonic
   * UnitsLoadFinishTimestampMonotonic
   * InitRDSecurityStartTimestampMonotonic
   * InitRDSecurityFinishTimestampMonotonic
   * InitRDGeneratorsStartTimestampMonotonic
   * InitRDGeneratorsFinishTimestampMonotonic
   * InitRDUnitsLoadStartTimestampMonotonic
   * InitRDUnitsLoadFinishTimestampMonotonic

All values are uint64's and are measured in microseconds.

## Unit Activation/Deactivation Timestamps

For each unit in the system the following timestamps are produced:

   * Activating
   * Activated
   * Deactivating
   * Deactivated
   * Time

The "Time" timestamp is the delta between Activated and Activating OR between
Deactivated and Deactivating depending on which set of timestamps is non zero.
This corresponds to the amount of time that it took a unit to start or to stop.

All values are uint64's and are measured in microseconds.  These timestamps are
sent for all units each internal period even though they will only change if a
service is restarted during the systems lifetime.

## Configuration

   * unitpattern: A comma separated list of patterns to match unit names against.

   For example the following will report only units ending in .target.

   ```
   unitpattern = "*.target"
   ```
   
   For example the following will report only units ending in any of .mount or .service.

   ```
   unitpattern = "*.mount,*.service"
   ```

   * periodic: A bool which instructs the plugin periodically collect boot
     metrics. The default (false) is to only collect metrics once since these
     are boot time metrics.

   For example the following instructs the plugin to continuously report boot
   timestamp metrics:

   ```
   periodic = true
   ```
