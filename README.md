# Brimcap [![Tests][tests-img]][tests]

A command line utility for converting pcap data into the flexible, searchable
zng data format as seen in the [Brim desktop
app](https://github.com/brimsec/brim) and the [zq command line
utility](https://github.com/brimsec/zq).

## Install

To build from source, go version 1.16 or later is required.

To install the brimcap binary in `$GOPATH/bin`, clone this repo and execute
`make install`:

```
git clone https://github.com/brimsec/brimcap
cd brimcap
make install
```

The use of the default configuration for `brimcap analayze [pcapfile]` requires
having both zeek and suricata installed and configured in your shell's path.
MacOS users can install these using Homebrew:

```
brew install zeek suricata
```

[ci-img]: https://github.com/brimsec/brimcap/actions/workflows/ci.yaml/badge.svg
[ci]: https://github.com/brimsec/zq/actions/workflows/ci.yaml

