# brimcap [![CI][ci-img]][ci]

![Image of brimcap analyze](https://github.com/brimdata/brimcap/raw/main/brimcap.gif)

A command line utility for converting pcap data into the flexible, searchable
zng data format as seen in the [Brim desktop
app](https://github.com/brimdata/brim) and the [zq command line
utility](https://github.com/brimdata/zed).

## Quickstart

1. [Install brimcap](#install).
2. Have a pcap handy (or download a sample pcap from
   https://wiki.wireshark.org/SampleCaptures).
3. Run brimcap analyze: `brimcap analyze sample.pcap > sample.zng`
4. Explore with [zq](https://github.com/brimdata/zed): `zq -z "zeek=count(has(_path)), alerts=count(has(event_type='alert'))" logs.zng`

## Usage with Brim desktop app

For exploring data in a rich, ui-based experience, data from analyze
can be automatically loaded into the
[Brim desktop app](https://github.com/brimdata/brim) using the brimcap load
command (builds for linux, macOS and windows available)\*:

1. Have the brim desktop app running.
2. Execute brimcap load on a pcap: `brimcap load sample.pcap`

\* The integration isn't great right now, a better experience coming soon!


## Install

The prebuilt brimcap package can found in the [release
section](https://github.com/brimdata/brimcap/releases) of the brimcap Github
repo.

The release includes a special brimdata build of
[zeek](https://github.com/brimdata/zeek) and
[suricata](https://github.com/brimdata/build-suricata) that is preconfigured to
provide a good experience out of the box for brimcap analyze.

Unzip the artifact and add the brimcap directory to your $PATH environment
variable.

```
export PATH=$PATH:/Path/To/brimcap
```

## Build From Source

To build from source, Go version 1.16 or later is required.

To build the brimcap package, clone this repo and execute
`make build`:

```
git clone https://github.com/brimdata/brimcap
cd brimcap
make build
```

`make build` will download the brimdata prebuilt / preconfigured zeek and
suricata artifacts, compile the brimcap binary and package them into
build/dist.


[ci-img]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml/badge.svg
[ci]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml

