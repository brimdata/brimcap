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
