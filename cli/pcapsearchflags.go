package cli

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/brimdata/brimcap"
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zson"
	"go.uber.org/multierr"
)

type PcapSearchFlags struct {
	Search brimcap.Search

	ts       *tsArg
	duration time.Duration
	proto    string
	srcip    ipArg
	srcport  portArg
	dstip    ipArg
	dstport  portArg
}

func (f *PcapSearchFlags) SetFlags(fs *flag.FlagSet) {
	f.ts = new(tsArg)
	fs.Var(f.ts, "ts", "the starting time stamp of the connection")
	fs.Func("duration", "the duration of the connection (default 1ns)", func(s string) error {
		if s == "" {
			f.duration = 1
			return nil
		}
		val, err := zson.ParseValue(zed.NewContext(), s)
		if err != nil {
			return err
		}
		if val.Type() != zed.TypeDuration {
			return fmt.Errorf("expected type %s got type %s", zson.FormatType(zed.TypeDuration), zson.FormatType(val.Type()))
		}
		f.duration = time.Duration(zed.DecodeDuration(val.Bytes()))
		return nil
	})
	fs.StringVar(&f.proto, "proto", "", "protocol of the connection (either tcp, udp or icmp)")
	fs.Var(&f.srcip, "src.ip", "ip address of the connection source")
	fs.Var(&f.srcport, "src.port", "port of the connection source")
	fs.Var(&f.dstip, "dst.ip", "ip address of the connection destination")
	fs.Var(&f.dstport, "dst.port", "port of the connection destination")
}

func (f *PcapSearchFlags) Init() error {
	var merr error
	if f.ts == nil {
		merr = multierr.Append(merr, errFlagRequired("-start"))
	}
	if f.srcip == nil {
		merr = multierr.Append(merr, errFlagRequired("-src.ip"))
	}
	if f.dstip == nil {
		merr = multierr.Append(merr, errFlagRequired("-dst.ip"))
	}
	switch f.proto {
	case "tcp", "udp", "icmp":
	case "":
		merr = multierr.Append(merr, errFlagRequired("-proto"))
	default:
		merr = multierr.Append(merr, fmt.Errorf("unsupported value for %q: %q", "-proto", f.proto))
	}
	if merr != nil {
		return merr
	}
	f.Search = brimcap.Search{
		Span:    nano.Span{Ts: nano.Ts(*f.ts), Dur: nano.Duration(f.duration)},
		Proto:   f.proto,
		SrcIP:   net.IP(f.srcip),
		SrcPort: uint16(f.srcport),
		DstIP:   net.IP(f.dstip),
		DstPort: uint16(f.dstport),
	}
	return nil
}

func errFlagRequired(flag string) error {
	return fmt.Errorf("%q required", flag)
}

type tsArg nano.Ts

func (t *tsArg) String() string {
	if t == nil {
		return ""
	}
	return nano.Ts(*t).String()
}

func (t *tsArg) Set(s string) error {
	out, err := nano.ParseRFC3339Nano([]byte(s))
	*t = tsArg(out)
	return err
}

type ipArg net.IP

func (i ipArg) String() string {
	if i == nil {
		return ""
	}
	return net.IP(i).String()
}

func (i *ipArg) Set(s string) error {
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("invalid IP value: %q", s)
	}
	*i = ipArg(ip)
	return nil
}

type portArg uint16

func (i portArg) String() string {
	if i == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(i), 10)
}

func (i *portArg) Set(s string) (err error) {
	val, err := strconv.ParseUint(s, 10, 16)
	*i = portArg(val)
	return err
}
