## v1.4.0
* Advance Zed dependency to include recent fixes/enhancements

## v1.3.0
* Update bundled Suricata to allow [use of local rules](https://github.com/brimdata/brimcap/issues/259) (#272, #274)

## v1.2.0
* `brimcap search`: parse `-duration` argument as a ZSON duration (#244)
* `brimcap slice`: parse `-to` and `-from` arguments as an RFC 3339 timestamp (#243)
* `brimcap ts`: print timestamps in RFC 3339 format (#243)
* Remove `brimcap migrate` (#234)

## v1.1.2
* Allow Brimcap analyzers to benefit from Zed JSON reader enhancements [zed/3124](https://github.com/brimdata/zed/pull/3124) and [zed/3123](https://github.com/brimdata/zed/pull/3123)

## v1.1.1
* Fix an issue where pcap index entries for legacy Spaces were not being migrated (#156)

## v1.1.0
* Allow expansion of environment variables in Brimcap config YAML (#153)

## v1.0.4
* Additions to custom YAML configuration (#148)
   * A `root` option can be used to specify the Brimcap root location
   * `name` is now a required part of an `analyzer` configuration

## v1.0.3
* Update legacy Space migration to work with Zed Lake branches (#140, #145)
* Fix an issue where temporary analyzer directories were not being deleted on exit (#137)

## v1.0.2
* Fix an issue where legacy Space migration would fail for a custom Data Directory in Brim (#133)

## v1.0.1
* Fix an issue where stale packet index entries could cause a failure to extract a flow from another pcap (#128)

## v1.0.0
* Include the name of the analyzer process with any warnings & errors it generates (#122)
* Adjust defaults for whether logging during analysis is output as JSON vs. status line (#123)
* Rather than quitting, emit a warning and continue if `brimcap analyze` fails to read an output file (#125)

## v0.0.6
* Move the [Geolocation article](https://github.com/brimdata/brimcap/wiki/Geolocation) over from the Brimcap wiki (#104)
* Refactor `brimcap analyze` to use the new Zed Lake add/commit endpoints and fix a deadlock issue (#110)
* Fix a deadlock issue that was caused by an analyze process writing no records (#115)
* Fix the percentage and byte counts on the command line status updates (#116)
* Drop `brimcap load` in favor of granular use of `brimcap analyze`, `brimcap index` and `zapi` (#117, #114, #120)

## v0.0.5
* Publish [Custom Brimcap Configuration](https://github.com/brimdata/brimcap/wiki/Custom-Brimcap-Config) wiki article (#72)
* Update the README (#96)
* Change `.` to `this` in Suricata shaper (#92)
* Fix an issue loading pcaps on some Linux distributions by using new Suricata artifact v5.0.3-brim2 (#100)

## v0.0.4
* Fix an issue where Space migrations could fail on Windows (#79)
* Generate an error message during abort of Space migration (#86)
* Create a [pcap troubleshooting wiki article ](https://github.com/brimdata/brimcap/wiki/Troubleshooting#ive-clicked-to-open-a-packet-capture-in-brim-but-it-failed-to-open) that includes info formerly from the [Brim wiki](https://github.com/brimdata/brim/wiki) (#83)
* Add SIGTERM to the list of signals Brimcap listens to for graceful shutdown (#88)
* Keep aborted Spaces so the caller of Brimcap (i.e., the Brim app) can handle cleanup (#89)
* Have Brimcap start using the new Zed Lake client (#90)

## v0.0.3

* Remove the `brimcap launch` command, since Brim will do `brimcap search` (#56)
* Adjust `brimcap load` to use the endpoints in `zed lake serve` (#63)
* Fix an issue with `pcap_path` not being stored as an absolute path, which caused problems extracting flows (#67)
* Add the hidden `brimcap migrate` command which Brim will use for migrating legacy Space data (#66)

## v0.0.2

* Fix an issue where use of symlinks in the root was preventing `brimcap load` from working on Windows (#39)

## v0.0.1

* Initial release, still being tested.
