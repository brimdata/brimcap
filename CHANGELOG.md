## v0.0.3

* Remove the `brimcap launch` command, since Brim will do `brimcap search` (#56)
* Adjust `brimcap load` to use the endpoints in `zed lake serve` (#63)
* Fix an issue with `pcap_path` not being stored as an absolute path, which caused problems extracting flows (#67)
* Add the hidden `brimcap migrate` command which Brim will use for migrating legacy Space data (#66)

## v0.0.2

* Fix an issue where use of symlinks in the root was preventing `brimcap load` from working on Windows (#39)

## v0.0.1

* Initial release, still being tested.
