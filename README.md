# Brimcap [![Tests][tests-img]][tests]

A command line utility for converting pcap data into the flexible, searchable
zng data format as seen in the [Brim desktop
app](https://github.com/brimsec/brim) and the [zq command line
utility](https://github.com/brimsec/zq).

## Quickstart

1. [Install brimcap (and dependencies)](#install).
2. Have a pcap handy (or download a sample pcap from
   https://wiki.wireshark.org/SampleCaptures).
3. Run brimcap analyze: `brimcap analyze sample.pcap > sample.zng`
4. Explore with [zq](https://github.com/brimdata/zq): `zq -z "zeek=count(has(_path)), alerts=count(has(event_type='alert'))" logs.zng`

## Usage with Brim desktop app

For exploring data in a rich, ui-based experience, data from analyze
can be automatically loaded into the
[Brim desktop app](https://github.com/brimdata/brim) using the brimcap load
command (builds for linux, macOS and windows available)\*:

1. Have the brim desktop app running.
2. Execute brimcap load on a pcap: `brimcap load sample.pcap`

\* The integration isn't great right now, a better experience coming soon!


## Install

To build from source, Go version 1.16 or later is required.

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

[ci-img]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml/badge.svg
[ci]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml
