# brimcap [![CI][ci-img]][ci]

![Image of brimcap analyze](https://github.com/brimdata/brimcap/raw/main/brimcap.gif)

A command line utility for converting [pcaps](https://en.wikipedia.org/wiki/Pcap#:~:text=In%20the%20field%20of%20computer,not%20the%20API's%20proper%20name.)
into the flexible, searchable [Zed data formats](https://zed.brimdata.io/docs/formats/)
as seen in the [Zui desktop app](https://github.com/brimdata/zui) and
[Zed commands](https://zed.brimdata.io/docs/commands/).

## Quickstart

1. [Install brimcap](#standalone-install)
2. Have a pcap handy (or download a [sample pcap](https://gitlab.com/wireshark/wireshark/-/wikis/SampleCaptures))
3. Run `brimcap analyze`
   ```
   brimcap analyze sample.pcap > sample.zng
   ```
4. Explore with [`zq`](https://zed.brimdata.io/docs/commands/zq/)
   ```
   zq -z 'zeek:=count(has(_path)), alerts:=count(has(event_type=="alert"))' sample.zng
   ```

## Usage with Zui desktop app

brimcap is bundled with the [Zui desktop app](https://github.com/brimdata/zui).
Whenever a pcap is imported into Zui, the app takes the following steps:

1. `brimcap analyze` is invoked to generate logs from the pcap.

2. The logs are imported into a newly-created pool in Zui's
   [Zed lake](https://zed.brimdata.io/docs/commands/zed/#1-the-lake-model).

3. `brimcap index` is invoked to populate a local pcap index that allows for
   quick extraction of flows via Zui's **Packets** button, which the app
   performs by invoking `brimcap search`.

If Zui is running, you can perform these same  operations from your shell,
which may prove useful for automation or batch import of many pcaps to the same
pool. The [Custom Brimcap Config](https://github.com/brimdata/brimcap/wiki/Custom-Brimcap-Config)
article shows example command lines along with other advanced configuration
options. When used with Zui, you should typically use the `brimcap` binary
found in Zui's `zdeps` directory (as described in the article), since this
version should be API-compatible with that version of Zui and its Zed backend.

## Brimcap Queries

Included in this repo is a `queries.json` file with some helpful queries for getting
started and exploring Zeek and Suricata analyzed data within the Zui app.

To import these queries:

1. Download the [`queries.json`](./queries.json?raw=1) file to your local system
2. In Zui, click the **+** menu in the upper-left corner of the app window and select **Import Queries...**
3. Open the downloaded file in the file picker utility

The loaded queries will appear in the "QUERIES" tab of Zui's left sidebar as a new folder named `Brimcap`.

## Standalone Install

If you're working with brimcap separate from the Zui app, prebuilt packages
can be found in the [releases section](https://github.com/brimdata/brimcap/releases)
of the brimcap GitHub repo.

Unzip the artifact and add the brimcap directory to your `$PATH` environment
variable.

```
export PATH="$PATH:/Path/To/brimcap"
```

## Included Analyzers

brimcap includes special builds of [Zeek](https://github.com/brimdata/build-zeek)
and [Suricata](https://github.com/brimdata/build-suricata) that were created by
the core development team at Brim Data. These builds are preconfigured to
provide a good experience out-of-the-box for generating logs from pcaps using
brimcap. If you wish to use your own customized Zeek/Suricata or introduce
other pcap analysis tools, this is described in the [Custom Brimcap
Config](https://github.com/brimdata/brimcap/wiki/Custom-Brimcap-Config) article.

## Build From Source

To build from source, Go 1.21 or later is required.

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
