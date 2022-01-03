# brimcap [![CI][ci-img]][ci]

![Image of brimcap analyze](https://github.com/brimdata/brimcap/raw/main/brimcap.gif)

A command line utility for converting [pcaps](https://en.wikipedia.org/wiki/Pcap#:~:text=In%20the%20field%20of%20computer,not%20the%20API's%20proper%20name.)
into the flexible, searchable [Zed data formats](https://github.com/brimdata/zed/tree/main/docs/data-model/README.md)
as seen in the [Brim desktop app](https://github.com/brimdata/brim) and
the [`zq` command line utility](https://github.com/brimdata/zed/tree/main/cmd/zed#zq).

## Quickstart

1. [Install brimcap](#standalone-install)
2. Have a pcap handy (or download a [sample pcap](https://gitlab.com/wireshark/wireshark/-/wikis/SampleCaptures))
3. Run `brimcap analyze`

   ```
   brimcap analyze sample.pcap > sample.zng
   ```
4. Explore with [zq](https://github.com/brimdata/zed/tree/main/cmd/zed#zq)
   ```
   zq -z 'zeek:=count(has(_path)), alerts:=count(has(event_type=="alert"))' sample.zng
   ```

## Usage with Brim desktop app

brimcap is bundled with the [Brim desktop app](https://github.com/brimdata/brim).
Whenever a pcap is imported into Brim, the app takes the following steps:

1. `brimcap analyze` is invoked to generate logs from the pcap.

2. The logs are imported into a newly-created
   [Pool in the Zed Lake](https://github.com/brimdata/zed/blob/main/docs/lake/README.md)
   behind Brim, similar to how `zapi create` and `zapi load` are used.

3. `brimcap index` is invoked to populate a local pcap index that allows for
   quick extraction of flows via Brim's **Packets** button, which the app
   performs by invoking `brimcap search`.

If Brim is running, you can perform these same  operations from your shell,
which may prove useful for automation or batch import of many pcaps to the same
Pool. The [Custom Brimcap Config](https://github.com/brimdata/brimcap/wiki/Custom-Brimcap-Config)
article shows example command lines along with other advanced configuration
options. When used with Brim, you should typically use the `brimcap` binary
found in Brim's `zdeps` directory (as described in the article), since this
version should be API-compatible with that version of Brim and its Zed backend.

## Standalone Install

If you're working with brimcap separate from the Brim app, prebuilt packages
can be found in the [releases section](https://github.com/brimdata/brimcap/releases)
of the brimcap GitHub repo.

Unzip the artifact and add the brimcap directory to your `$PATH` environment
variable.

```
export PATH="$PATH:/Path/To/brimcap"
```

## Included Analyzers

brimcap includes special builds of [Zeek](https://github.com/brimdata/zeek)
and [Suricata](https://github.com/brimdata/build-suricata) that were created by
the core development team at Brim Data. These builds are preconfigured to
provide a good experience out-of-the-box for generating logs from pcaps using
brimcap. If you wish to use your own customized Zeek/Suricata or introduce
other pcap analysis tools, this is described in the [Custom Brimcap
Config](https://github.com/brimdata/brimcap/wiki/Custom-Brimcap-Config) article.

## Build From Source

To build from source, Go version 1.16 or later is required.

To build the brimcap package, clone this repo and run `make build`:

```
git clone https://github.com/brimdata/brimcap
cd brimcap
make build
```

`make build` will download the prebuilt/preconfigured Zeek and Suricata
artifacts, compile the brimcap binary and package them into `build/dist`.

The executables will be located here:
```
./build/dist/brimcap
./build/dist/zeek/zeekrunner
./build/dist/suricata/suricatarunner
```

## Having a problem?

Please browse the [wiki](https://github.com/brimdata/brimcap/wiki) to review common problems and helpful tips before [opening an issue](https://github.com/brimdata/brimcap/wiki/Troubleshooting#opening-an-issue).

## Join the Community

Join our [Public Slack](https://www.brimdata.io/join-slack/) workspace for announcements, Q&A, and to trade tips!

[ci-img]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml/badge.svg
[ci]: https://github.com/brimdata/brimcap/actions/workflows/ci.yaml
